// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package explorer

import (
	"context"
	"encoding/hex"
	"math/big"

	"github.com/golang/protobuf/proto"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol/multichain/mainchain"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/dispatcher"
	"github.com/iotexproject/iotex-core/explorer/idl/explorer"
	"github.com/iotexproject/iotex-core/indexservice"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/protogen/iotextypes"
)

var (
	// ErrInternalServer indicates the internal server error
	ErrInternalServer = errors.New("internal server error")
	// ErrTransfer indicates the error of transfer
	ErrTransfer = errors.New("invalid transfer")
	// ErrVote indicates the error of vote
	ErrVote = errors.New("invalid vote")
	// ErrExecution indicates the error of execution
	ErrExecution = errors.New("invalid execution")
	// ErrReceipt indicates the error of receipt
	ErrReceipt = errors.New("invalid receipt")
	// ErrAction indicates the error of action
	ErrAction = errors.New("invalid action")
)

var (
	requestMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_explorer_request",
			Help: "IoTeX Explorer request counter.",
		},
		[]string{"method", "succeed"},
	)
)

const (
	trueStr  = "ture"
	falseStr = "false"
)

func init() {
	prometheus.MustRegister(requestMtc)
}

type (
	// BroadcastOutbound sends a broadcast message to the whole network
	BroadcastOutbound func(ctx context.Context, chainID uint32, msg proto.Message) error
	// Neighbors returns the neighbors' addresses
	Neighbors func(context.Context) ([]peerstore.PeerInfo, error)
	// NetworkInfo returns the self network information
	NetworkInfo func() peerstore.PeerInfo
)

// Service provide api for user to query blockchain data
type Service struct {
	bc                 blockchain.Blockchain
	c                  consensus.Consensus
	dp                 dispatcher.Dispatcher
	ap                 actpool.ActPool
	gs                 GasStation
	broadcastHandler   BroadcastOutbound
	neighborsHandler   Neighbors
	networkInfoHandler NetworkInfo
	cfg                config.Explorer
	idx                *indexservice.Server
	// TODO: the way to make explorer to access the data model managed by main-chain protocol is hack. We need to
	// refactor the code later
	mainChain *mainchain.Protocol
}

// SetMainChainProtocol sets the main-chain side multi-chain protocol
func (exp *Service) SetMainChainProtocol(mainChain *mainchain.Protocol) { exp.mainChain = mainChain }

// GetAddressDetails returns the properties of an address
func (exp *Service) GetAddressDetails(address string) (explorer.AddressDetails, error) {
	state, err := exp.bc.StateByAddr(address)
	if err != nil {
		return explorer.AddressDetails{}, err
	}
	pendingNonce, err := exp.ap.GetPendingNonce(address)
	if err != nil {
		return explorer.AddressDetails{}, err
	}
	details := explorer.AddressDetails{
		Address:      address,
		TotalBalance: state.Balance.String(),
		Nonce:        int64(state.Nonce),
		PendingNonce: int64(pendingNonce),
		IsCandidate:  state.IsCandidate,
	}

	return details, nil
}

// PutSubChainBlock put block merkel root on root chain.
func (exp *Service) PutSubChainBlock(putBlockJSON explorer.PutSubChainBlockRequest) (resp explorer.PutSubChainBlockResponse, err error) {
	log.L().Debug("receive put block request")

	defer func() {
		succeed := trueStr
		if err != nil {
			succeed = falseStr
		}
		requestMtc.WithLabelValues("PutBlock", succeed).Inc()
	}()

	senderPubKey, err := keypair.StringToPubKeyBytes(putBlockJSON.SenderPubKey)
	if err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	signature, err := hex.DecodeString(putBlockJSON.Signature)
	if err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	gasPrice, ok := big.NewInt(0).SetString(putBlockJSON.GasPrice, 10)
	if !ok {
		return explorer.PutSubChainBlockResponse{}, errors.New("failed to set vote gas price")
	}

	roots := make([]*iotextypes.MerkleRoot, 0)
	for _, mr := range putBlockJSON.Roots {
		v, err := hex.DecodeString(mr.Value)
		if err != nil {
			return explorer.PutSubChainBlockResponse{}, err
		}
		roots = append(roots, &iotextypes.MerkleRoot{
			Name:  mr.Name,
			Value: v,
		})
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_PutBlock{
				PutBlock: &iotextypes.PutBlock{
					SubChainAddress: putBlockJSON.SubChainAddress,
					Height:          uint64(putBlockJSON.Height),
					Roots:           roots,
				},
			},
			Version:  uint32(putBlockJSON.Version),
			Nonce:    uint64(putBlockJSON.Nonce),
			GasLimit: uint64(putBlockJSON.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: senderPubKey,
		Signature:    signature,
	}
	// broadcast to the network
	if err := exp.broadcastHandler(context.Background(), exp.bc.ChainID(), actPb); err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	// send to actpool via dispatcher
	exp.dp.HandleBroadcast(context.Background(), exp.bc.ChainID(), actPb)

	v := &action.SealedEnvelope{}
	if err := v.LoadProto(actPb); err != nil {
		return explorer.PutSubChainBlockResponse{}, err
	}
	h := v.Hash()
	return explorer.PutSubChainBlockResponse{Hash: hex.EncodeToString(h[:])}, nil
}

// GetDeposits returns the deposits of a sub-chain in the given range in descending order by the index
func (exp *Service) GetDeposits(subChainID int64, offset int64, limit int64) ([]explorer.Deposit, error) {
	subChainsInOp, err := exp.mainChain.SubChainsInOperation()
	if err != nil {
		return nil, err
	}
	var targetSubChain mainchain.InOperation
	for _, subChainInOp := range subChainsInOp {
		if subChainInOp.ID == uint32(subChainID) {
			targetSubChain = subChainInOp
		}
	}
	if targetSubChain.ID != uint32(subChainID) {
		return nil, errors.Errorf("sub-chain %d is not found in operation", subChainID)
	}
	subChainAddr, err := address.FromBytes(targetSubChain.Addr)
	if err != nil {
		return nil, err
	}
	subChain, err := exp.mainChain.SubChain(subChainAddr)
	if err != nil {
		return nil, err
	}
	idx := uint64(offset)
	// If the last deposit index is lower than the start index, reset it
	if subChain.DepositCount-1 < idx {
		idx = subChain.DepositCount - 1
	}
	var deposits []explorer.Deposit
	for count := int64(0); count < limit; count++ {
		deposit, err := exp.mainChain.Deposit(subChainAddr, idx)
		if err != nil {
			return nil, err
		}
		recipient, err := address.FromBytes(deposit.Addr)
		if err != nil {
			return nil, err
		}
		deposits = append(deposits, explorer.Deposit{
			Amount:    deposit.Amount.String(),
			Address:   recipient.String(),
			Confirmed: deposit.Confirmed,
		})
		if idx > 0 {
			idx--
		} else {
			break
		}
	}
	return deposits, nil
}

// GetCandidateMetricsByHeight returns the candidates metrics for given height.
func (exp *Service) GetCandidateMetricsByHeight(h int64) (explorer.CandidateMetrics, error) {
	if h < 0 {
		return explorer.CandidateMetrics{}, errors.New("Invalid height")
	}
	allCandidates, err := exp.bc.CandidatesByHeight(uint64(h))
	if err != nil {
		return explorer.CandidateMetrics{}, errors.Wrapf(err,
			"Failed to get the candidate metrics")
	}
	candidates := make([]explorer.Candidate, 0, len(allCandidates))
	for _, c := range allCandidates {
		candidates = append(candidates, explorer.Candidate{
			Address:   c.Address,
			TotalVote: c.Votes.String(),
		})
	}

	return explorer.CandidateMetrics{
		Candidates: candidates,
	}, nil
}
func convertExplorerExecutionToActionPb(execution *explorer.Execution) (*iotextypes.Action, error) {
	executorPubKey, err := keypair.StringToPubKeyBytes(execution.ExecutorPubKey)
	if err != nil {
		return nil, err
	}
	data, err := hex.DecodeString(execution.Data)
	if err != nil {
		return nil, err
	}
	signature, err := hex.DecodeString(execution.Signature)
	if err != nil {
		return nil, err
	}
	amount, ok := big.NewInt(0).SetString(execution.Amount, 10)
	if !ok {
		return nil, errors.New("failed to set execution amount")
	}
	gasPrice, ok := big.NewInt(0).SetString(execution.GasPrice, 10)
	if !ok {
		return nil, errors.New("failed to set execution gas price")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Execution{
				Execution: &iotextypes.Execution{
					Amount:   amount.String(),
					Contract: execution.Contract,
					Data:     data,
				},
			},
			Version:  uint32(execution.Version),
			Nonce:    uint64(execution.Nonce),
			GasLimit: uint64(execution.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: executorPubKey,
		Signature:    signature,
	}
	return actPb, nil
}

func convertExplorerTransferToActionPb(tsfJSON *explorer.SendTransferRequest,
	maxTransferPayloadBytes uint64) (*iotextypes.Action, error) {
	payload, err := hex.DecodeString(tsfJSON.Payload)
	if err != nil {
		return nil, err
	}
	if uint64(len(payload)) > maxTransferPayloadBytes {
		return nil, errors.Wrapf(
			ErrTransfer,
			"transfer payload contains %d bytes, and is longer than %d bytes limit",
			len(payload),
			maxTransferPayloadBytes,
		)
	}
	senderPubKey, err := keypair.StringToPubKeyBytes(tsfJSON.SenderPubKey)
	if err != nil {
		return nil, err
	}
	signature, err := hex.DecodeString(tsfJSON.Signature)
	if err != nil {
		return nil, err
	}
	amount, ok := big.NewInt(0).SetString(tsfJSON.Amount, 10)
	if !ok {
		return nil, errors.New("failed to set transfer amount")
	}
	gasPrice, ok := big.NewInt(0).SetString(tsfJSON.GasPrice, 10)
	if !ok {
		return nil, errors.New("failed to set transfer gas price")
	}
	actPb := &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{
					Amount:    amount.String(),
					Recipient: tsfJSON.Recipient,
					Payload:   payload,
				},
			},
			Version:  uint32(tsfJSON.Version),
			Nonce:    uint64(tsfJSON.Nonce),
			GasLimit: uint64(tsfJSON.GasLimit),
			GasPrice: gasPrice.String(),
		},
		SenderPubKey: senderPubKey,
		Signature:    signature,
	}
	return actPb, nil
}
