// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/gasstation"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/systemlog"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/testutil"
)

const lld = "lifeLongDelegates"

var (
	testTransfer, _ = testutil.SignedTransfer(identityset.Address(28).String(),
		identityset.PrivateKey(28), 3, big.NewInt(10), []byte{}, testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64))

	testTransferHash = testTransfer.Hash()
	testTransferPb   = testTransfer.Proto()

	testExecution, _ = testutil.SignedExecution(identityset.Address(29).String(),
		identityset.PrivateKey(29), 1, big.NewInt(0), testutil.TestGasLimit,
		big.NewInt(testutil.TestGasPriceInt64), []byte{})

	testExecutionHash = testExecution.Hash()
	testExecutionPb   = testExecution.Proto()

	testTransfer1, _ = testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(27), 1,
		big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash1    = testTransfer1.Hash()
	testTransfer2, _ = testutil.SignedTransfer(identityset.Address(30).String(), identityset.PrivateKey(30), 5,
		big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	transferHash2 = testTransfer2.Hash()

	testExecution1, _ = testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash1 = testExecution1.Hash()

	testExecution2, _ = testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(30), 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash2 = testExecution2.Hash()

	testExecution3, _ = testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(28), 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	executionHash3 = testExecution3.Hash()

	testReceiptWithSystemLog = &action.Receipt{
		Status:          1,
		BlockHeight:     1,
		ActionHash:      testExecution.Hash(),
		GasConsumed:     0,
		ContractAddress: identityset.Address(31).String(),
		Logs: []*action.Log{
			{
				Address: identityset.Address(31).String(),
				Topics: []hash.Hash256{
					hash.BytesToHash256(action.InContractTransfer[:]),
					hash.BytesToHash256(identityset.Address(30).Bytes()),
					hash.BytesToHash256(identityset.Address(29).Bytes()),
				},
				Data:        big.NewInt(3).Bytes(),
				BlockHeight: 1,
				ActionHash:  testExecution.Hash(),
			},
		},
	}
)

var (
	delegates = []genesis.Delegate{
		{
			OperatorAddrStr: identityset.Address(0).String(),
			VotesStr:        "10",
		},
		{
			OperatorAddrStr: identityset.Address(1).String(),
			VotesStr:        "10",
		},
		{
			OperatorAddrStr: identityset.Address(2).String(),
			VotesStr:        "10",
		},
	}
)

var (
	getAccountTests = []struct {
		in           string
		address      string
		balance      string
		nonce        uint64
		pendingNonce uint64
		numActions   uint64
	}{
		{identityset.Address(30).String(),
			"io1d4c5lp4ea4754wy439g2t99ue7wryu5r2lslh2",
			"3",
			8,
			9,
			9,
		},
		{
			identityset.Address(27).String(),
			"io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms",
			"9999999999999999999999999991",
			1,
			6,
			2,
		},
	}

	getActionsTests = []struct {
		start      uint64
		count      uint64
		numActions int
	}{
		{
			1,
			11,
			11,
		},
		{
			11,
			5,
			4,
		},
		{
			1,
			0,
			0,
		},
	}

	getActionTests = []struct {
		// Arguments
		checkPending bool
		in           string
		// Expected Values
		nonce        uint64
		senderPubKey string
		blkNumber    uint64
	}{
		{
			checkPending: false,
			in:           hex.EncodeToString(transferHash1[:]),
			nonce:        1,
			senderPubKey: testTransfer1.SrcPubkey().HexString(),
			blkNumber:    1,
		},
		{
			checkPending: false,
			in:           hex.EncodeToString(transferHash2[:]),
			nonce:        5,
			senderPubKey: testTransfer2.SrcPubkey().HexString(),
			blkNumber:    2,
		},
		{
			checkPending: false,
			in:           hex.EncodeToString(executionHash1[:]),
			nonce:        6,
			senderPubKey: testExecution1.SrcPubkey().HexString(),
			blkNumber:    2,
		},
	}

	getActionsByAddressTests = []struct {
		address    string
		start      uint64
		count      uint64
		numActions int
	}{
		{
			identityset.Address(27).String(),
			0,
			3,
			2,
		},
		{
			identityset.Address(30).String(),
			1,
			8,
			8,
		},
		{
			identityset.Address(33).String(),
			2,
			1,
			0,
		},
	}

	getUnconfirmedActionsByAddressTests = []struct {
		address    string
		start      uint64
		count      uint64
		numActions int
	}{
		{
			identityset.Address(27).String(),
			0,
			4,
			4,
		},
		{
			identityset.Address(27).String(),
			2,
			0,
			0,
		},
	}

	getActionsByBlockTests = []struct {
		blkHeight  uint64
		start      uint64
		count      uint64
		numActions int
	}{
		{
			2,
			0,
			7,
			7,
		},
		{
			4,
			0,
			5,
			5,
		},
		{
			1,
			0,
			0,
			0,
		},
	}

	getBlockMetasTests = []struct {
		start   uint64
		count   uint64
		numBlks int
	}{
		{
			1,
			4,
			4,
		},
		{
			2,
			5,
			3,
		},
		{
			1,
			0,
			0,
		},
	}

	getBlockMetaTests = []struct {
		blkHeight      uint64
		numActions     int64
		transferAmount string
		logsBloom      string
	}{
		{
			2,
			7,
			"6",
			"",
		},
		{
			4,
			5,
			"2",
			"",
		},
	}

	getChainMetaTests = []struct {
		// Arguments
		emptyChain       bool
		tpsWindow        int
		pollProtocolType string
		// Expected values
		height     uint64
		numActions int64
		tps        int64
		tpsFloat   float32
		epoch      iotextypes.EpochData
	}{
		{
			emptyChain: true,
		},

		{
			false,
			1,
			lld,
			4,
			15,
			1,
			5 / 10.0,
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 1,
			},
		},
		{
			false,
			5,
			"governanceChainCommittee",
			4,
			15,
			2,
			15 / 13.0,
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 100,
			},
		},
	}

	sendActionTests = []struct {
		// Arguments
		actionPb *iotextypes.Action
		// Expected Values
		actionHash string
	}{
		{
			testTransferPb,
			hex.EncodeToString(testTransferHash[:]),
		},
		{
			testExecutionPb,
			hex.EncodeToString(testExecutionHash[:]),
		},
	}

	getReceiptByActionTests = []struct {
		in        string
		status    uint64
		blkHeight uint64
	}{
		{
			hex.EncodeToString(transferHash1[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			1,
		},
		{
			hex.EncodeToString(transferHash2[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			2,
		},
		{
			hex.EncodeToString(executionHash2[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			2,
		},
		{
			hex.EncodeToString(executionHash3[:]),
			uint64(iotextypes.ReceiptStatus_Success),
			4,
		},
	}

	readContractTests = []struct {
		execHash   string
		callerAddr string
		retValue   string
	}{
		{
			hex.EncodeToString(executionHash2[:]),
			identityset.Address(30).String(),
			"",
		},
	}

	suggestGasPriceTests = []struct {
		defaultGasPrice   uint64
		suggestedGasPrice uint64
	}{
		{
			1,
			1,
		},
	}

	estimateGasForActionTests = []struct {
		actionHash   string
		estimatedGas uint64
	}{
		{
			hex.EncodeToString(transferHash1[:]),
			10000,
		},
		{
			hex.EncodeToString(transferHash2[:]),
			10000,
		},
	}

	readUnclaimedBalanceTests = []struct {
		// Arguments
		protocolID string
		methodName string
		addr       string
		// Expected values
		returnErr bool
		balance   *big.Int
	}{
		{
			protocolID: "rewarding",
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(0).String(),
			returnErr:  false,
			balance:    unit.ConvertIotxToRau(64), // 4 block * 36 IOTX reward by default = 144 IOTX
		},
		{
			protocolID: "rewarding",
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(1).String(),
			returnErr:  false,
			balance:    unit.ConvertIotxToRau(0), // 4 block * 36 IOTX reward by default = 144 IOTX
		},
		{
			protocolID: "Wrong ID",
			methodName: "UnclaimedBalance",
			addr:       identityset.Address(27).String(),
			returnErr:  true,
		},
		{
			protocolID: "rewarding",
			methodName: "Wrong Method",
			addr:       identityset.Address(27).String(),
			returnErr:  true,
		},
	}

	readCandidatesByEpochTests = []struct {
		// Arguments
		protocolID   string
		protocolType string
		methodName   string
		epoch        uint64
		// Expected Values
		numDelegates int
	}{
		{
			protocolID:   "poll",
			protocolType: lld,
			methodName:   "CandidatesByEpoch",
			epoch:        1,
			numDelegates: 3,
		},
		{
			protocolID:   "poll",
			protocolType: "governanceChainCommittee",
			methodName:   "CandidatesByEpoch",
			epoch:        1,
			numDelegates: 2,
		},
	}

	readBlockProducersByEpochTests = []struct {
		// Arguments
		protocolID            string
		protocolType          string
		methodName            string
		epoch                 uint64
		numCandidateDelegates uint64
		// Expected Values
		numBlockProducers int
	}{
		{
			protocolID:        "poll",
			protocolType:      lld,
			methodName:        "BlockProducersByEpoch",
			epoch:             1,
			numBlockProducers: 3,
		},
		{
			protocolID:            "poll",
			protocolType:          "governanceChainCommittee",
			methodName:            "BlockProducersByEpoch",
			epoch:                 1,
			numCandidateDelegates: 2,
			numBlockProducers:     2,
		},
		{
			protocolID:            "poll",
			protocolType:          "governanceChainCommittee",
			methodName:            "BlockProducersByEpoch",
			epoch:                 1,
			numCandidateDelegates: 1,
			numBlockProducers:     1,
		},
	}

	readActiveBlockProducersByEpochTests = []struct {
		// Arguments
		protocolID   string
		protocolType string
		methodName   string
		epoch        uint64
		numDelegates uint64
		// Expected Values
		numActiveBlockProducers int
	}{
		{
			protocolID:              "poll",
			protocolType:            lld,
			methodName:              "ActiveBlockProducersByEpoch",
			epoch:                   1,
			numActiveBlockProducers: 3,
		},
		{
			protocolID:              "poll",
			protocolType:            "governanceChainCommittee",
			methodName:              "ActiveBlockProducersByEpoch",
			epoch:                   1,
			numDelegates:            2,
			numActiveBlockProducers: 2,
		},
		{
			protocolID:              "poll",
			protocolType:            "governanceChainCommittee",
			methodName:              "ActiveBlockProducersByEpoch",
			epoch:                   1,
			numDelegates:            1,
			numActiveBlockProducers: 1,
		},
	}

	readRollDPoSMetaTests = []struct {
		// Arguments
		protocolID string
		methodName string
		height     uint64
		// Expected Values
		result uint64
	}{
		{
			protocolID: "rolldpos",
			methodName: "NumCandidateDelegates",
			result:     36,
		},
		{
			protocolID: "rolldpos",
			methodName: "NumDelegates",
			result:     24,
		},
	}

	readEpochCtxTests = []struct {
		// Arguments
		protocolID string
		methodName string
		argument   uint64
		// Expected Values
		result uint64
	}{
		{
			protocolID: "rolldpos",
			methodName: "NumSubEpochs",
			argument:   1,
			result:     2,
		},
		{
			protocolID: "rolldpos",
			methodName: "NumSubEpochs",
			argument:   1816201,
			result:     30,
		},
		{
			protocolID: "rolldpos",
			methodName: "EpochNumber",
			argument:   100,
			result:     3,
		},
		{
			protocolID: "rolldpos",
			methodName: "EpochHeight",
			argument:   5,
			result:     193,
		},
		{
			protocolID: "rolldpos",
			methodName: "EpochLastHeight",
			argument:   1000,
			result:     48000,
		},
		{
			protocolID: "rolldpos",
			methodName: "SubEpochNumber",
			argument:   121,
			result:     1,
		},
	}

	getEpochMetaTests = []struct {
		// Arguments
		EpochNumber      uint64
		pollProtocolType string
		// Expected Values
		epochData                     iotextypes.EpochData
		numBlksInEpoch                int
		numConsenusBlockProducers     int
		numActiveCensusBlockProducers int
	}{
		{
			1,
			lld,
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 1,
			},
			4,
			24,
			24,
		},
		{
			1,
			"governanceChainCommittee",
			iotextypes.EpochData{
				Num:                     1,
				Height:                  1,
				GravityChainStartHeight: 100,
			},
			4,
			6,
			6,
		},
	}

	getRawBlocksTest = []struct {
		// Arguments
		startHeight  uint64
		count        uint64
		withReceipts bool
		// Expected Values
		numBlks     int
		numActions  int
		numReceipts int
	}{
		{
			1,
			1,
			false,
			1,
			2,
			0,
		},
		{
			1,
			2,
			true,
			2,
			9,
			9,
		},
	}

	getLogsTest = []struct {
		// Arguments
		address   []string
		topics    []*iotexapi.Topics
		fromBlock uint64
		count     uint64
		// Expected Values
		numLogs int
	}{
		{
			address:   []string{},
			topics:    []*iotexapi.Topics{},
			fromBlock: 1,
			count:     100,
			numLogs:   4,
		},
	}

	getEvmTransfersByActionHashTest = []struct {
		// Arguments
		actHash hash.Hash256
		// Expected Values
		numEvmTransfer uint64
		amount         [][]byte
		from           []string
		to             []string
	}{
		{
			actHash:        testExecution.Hash(),
			numEvmTransfer: uint64(1),
			amount:         [][]byte{big.NewInt(3).Bytes()},
			from:           []string{identityset.Address(30).String()},
			to:             []string{identityset.Address(29).String()},
		},
	}

	getEvmTransfersByBlockHeightTest = []struct {
		// Arguments
		height uint64
		// Expected Values
		numEvmTransfer uint64
		actTransfers   []struct {
			actHash        hash.Hash256
			numEvmTransfer uint64
			amount         [][]byte
			from           []string
			to             []string
		}
	}{
		{
			height:         uint64(1),
			numEvmTransfer: uint64(1),
			actTransfers: []struct {
				actHash        hash.Hash256
				numEvmTransfer uint64
				amount         [][]byte
				from           []string
				to             []string
			}{
				{
					actHash:        testExecution.Hash(),
					numEvmTransfer: uint64(1),
					amount:         [][]byte{big.NewInt(3).Bytes()},
					from:           []string{identityset.Address(30).String()},
					to:             []string{identityset.Address(29).String()},
				},
			},
		},
	}
)

func TestServer_GetAccount(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	// success
	for _, test := range getAccountTests {
		request := &iotexapi.GetAccountRequest{Address: test.in}
		res, err := svr.GetAccount(context.Background(), request)
		require.NoError(err)
		accountMeta := res.AccountMeta
		require.Equal(test.address, accountMeta.Address)
		require.Equal(test.balance, accountMeta.Balance)
		require.Equal(test.nonce, accountMeta.Nonce)
		require.Equal(test.pendingNonce, accountMeta.PendingNonce)
		require.Equal(test.numActions, accountMeta.NumActions)
	}
	// failure
	_, err = svr.GetAccount(context.Background(), &iotexapi.GetAccountRequest{})
	require.Error(err)
}

func TestServer_GetActions(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByIndex{
				ByIndex: &iotexapi.GetActionsByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
	}
}

func TestServer_GetAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	for _, test := range getActionTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByHash{
				ByHash: &iotexapi.GetActionByHashRequest{
					ActionHash:   test.in,
					CheckPending: test.checkPending,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.ActionInfo))
		act := res.ActionInfo[0]
		require.Equal(test.nonce, act.Action.GetCore().GetNonce())
		require.Equal(test.senderPubKey, hex.EncodeToString(act.Action.SenderPubKey))
		if !test.checkPending {
			blk, err := svr.dao.GetBlockByHeight(test.blkNumber)
			require.NoError(err)
			timeStamp := blk.ConvertToBlockHeaderPb().GetCore().GetTimestamp()
			blkHash := blk.HashBlock()
			require.Equal(hex.EncodeToString(blkHash[:]), act.BlkHash)
			require.Equal(test.blkNumber, act.BlkHeight)
			require.Equal(timeStamp, act.Timestamp)
		} else {
			require.Equal(hex.EncodeToString(hash.ZeroHash256[:]), act.BlkHash)
			require.Nil(act.Timestamp)
			require.Equal(uint64(0), act.BlkHeight)
		}
	}
}

func TestServer_GetActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsByAddressTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByAddr{
				ByAddr: &iotexapi.GetActionsByAddressRequest{
					Address: test.address,
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		if test.numActions == 0 {
			// returns empty response body in case of no result
			require.Equal(&iotexapi.GetActionsResponse{}, res)
		}
		var prevAct *iotexapi.ActionInfo
		for _, act := range res.ActionInfo {
			if prevAct != nil {
				require.True(act.Timestamp.GetSeconds() >= prevAct.Timestamp.GetSeconds())
			}
			prevAct = act
		}
		if test.start > 0 && len(res.ActionInfo) > 0 {
			request = &iotexapi.GetActionsRequest{
				Lookup: &iotexapi.GetActionsRequest_ByAddr{
					ByAddr: &iotexapi.GetActionsByAddressRequest{
						Address: test.address,
						Start:   0,
						Count:   test.start,
					},
				},
			}
			prevRes, err := svr.GetActions(context.Background(), request)
			require.NoError(err)
			require.True(prevRes.ActionInfo[len(prevRes.ActionInfo)-1].Timestamp.GetSeconds() <= res.ActionInfo[0].Timestamp.GetSeconds())
		}
	}
}

func TestServer_GetUnconfirmedActionsByAddress(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, true)
	require.NoError(err)

	for _, test := range getUnconfirmedActionsByAddressTests {
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_UnconfirmedByAddr{
				UnconfirmedByAddr: &iotexapi.GetUnconfirmedActionsByAddressRequest{
					Address: test.address,
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		require.Equal(test.address, res.ActionInfo[0].Sender)
	}
}

func TestServer_GetActionsByBlock(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getActionsByBlockTests {
		header, err := svr.bc.BlockHeaderByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := header.HashBlock()
		request := &iotexapi.GetActionsRequest{
			Lookup: &iotexapi.GetActionsRequest_ByBlk{
				ByBlk: &iotexapi.GetActionsByBlockRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
					Start:   test.start,
					Count:   test.count,
				},
			},
		}
		res, err := svr.GetActions(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numActions, len(res.ActionInfo))
		require.Equal(test.blkHeight, res.ActionInfo[0].BlkHeight)
	}
}

func TestServer_GetBlockMetas(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)
	require.NotNil(svr.indexer)

	for _, test := range getBlockMetasTests {
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByIndex{
				ByIndex: &iotexapi.GetBlockMetasByIndexRequest{
					Start: test.start,
					Count: test.count,
				},
			},
		}
		res, err := svr.GetBlockMetas(context.Background(), request)
		if test.count == 0 {
			require.Error(err)
			continue
		}
		require.NoError(err)
		require.Equal(test.numBlks, len(res.BlkMetas))
		var prevBlkPb *iotextypes.BlockMeta
		for _, blkPb := range res.BlkMetas {
			if prevBlkPb != nil {
				require.True(blkPb.Height > prevBlkPb.Height)
			}
			prevBlkPb = blkPb
		}
	}
}

func TestServer_GetBlockMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getBlockMetaTests {
		header, err := svr.bc.BlockHeaderByHeight(test.blkHeight)
		require.NoError(err)
		blkHash := header.HashBlock()
		request := &iotexapi.GetBlockMetasRequest{
			Lookup: &iotexapi.GetBlockMetasRequest_ByHash{
				ByHash: &iotexapi.GetBlockMetaByHashRequest{
					BlkHash: hex.EncodeToString(blkHash[:]),
				},
			},
		}
		res, err := svr.GetBlockMetas(context.Background(), request)
		require.NoError(err)
		require.Equal(1, len(res.BlkMetas))
		blkPb := res.BlkMetas[0]
		require.Equal(test.blkHeight, blkPb.Height)
		require.Equal(test.numActions, blkPb.NumActions)
		require.Equal(test.transferAmount, blkPb.TransferAmount)
		require.Equal(header.LogsBloomfilter(), nil)
		require.Equal(test.logsBloom, blkPb.LogsBloom)
	}
}

func TestServer_GetChainMeta(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	var pol poll.Protocol
	for _, test := range getChainMetaTests {
		cfg := newConfig(t)
		if test.pollProtocolType == lld {
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			slasher, _ := poll.NewSlasher(
				&cfg.Genesis,
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				nil,
				nil,
				nil,
				nil,
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				nil,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
			committee.EXPECT().HeightByTime(gomock.Any()).Return(test.epoch.GravityChainStartHeight, nil)
		}

		cfg.API.TpsWindow = test.tpsWindow
		svr, err := createServer(cfg, false)
		require.NoError(err)
		if pol != nil {
			require.NoError(pol.ForceRegister(svr.registry))
		}
		if test.emptyChain {
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			mbc.EXPECT().TipHeight().Return(uint64(0)).Times(1)
			svr.bc = mbc
		}
		res, err := svr.GetChainMeta(context.Background(), &iotexapi.GetChainMetaRequest{})
		require.NoError(err)
		chainMetaPb := res.ChainMeta
		require.Equal(test.height, chainMetaPb.Height)
		require.Equal(test.numActions, chainMetaPb.NumActions)
		require.Equal(test.tps, chainMetaPb.Tps)
		require.Equal(test.epoch.Num, chainMetaPb.Epoch.Num)
		require.Equal(test.epoch.Height, chainMetaPb.Epoch.Height)
		require.Equal(test.epoch.GravityChainStartHeight, chainMetaPb.Epoch.GravityChainStartHeight)
	}
}

func TestServer_SendAction(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	chain := mock_blockchain.NewMockBlockchain(ctrl)
	ap := mock_actpool.NewMockActPool(ctrl)
	broadcastHandlerCount := 0
	svr := Server{bc: chain, ap: ap, broadcastHandler: func(_ context.Context, _ uint32, _ proto.Message) error {
		broadcastHandlerCount++
		return nil
	}}

	chain.EXPECT().ChainID().Return(uint32(1)).Times(2)
	ap.EXPECT().Add(gomock.Any(), gomock.Any()).Return(nil).Times(2)

	for i, test := range sendActionTests {
		request := &iotexapi.SendActionRequest{Action: test.actionPb}
		res, err := svr.SendAction(context.Background(), request)
		require.NoError(err)
		require.Equal(i+1, broadcastHandlerCount)
		require.Equal(test.actionHash, res.ActionHash)
	}
}

func TestServer_GetReceiptByAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getReceiptByActionTests {
		request := &iotexapi.GetReceiptByActionRequest{ActionHash: test.in}
		res, err := svr.GetReceiptByAction(context.Background(), request)
		require.NoError(err)
		receiptPb := res.ReceiptInfo.Receipt
		require.Equal(test.status, receiptPb.Status)
		require.Equal(test.blkHeight, receiptPb.BlkHeight)
		require.NotEqual(hash.ZeroHash256, res.ReceiptInfo.BlkHash)
	}
}

func TestServer_ReadContract(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range readContractTests {
		hash, err := hash.HexStringToHash256(test.execHash)
		require.NoError(err)
		ai, err := svr.indexer.GetActionIndex(hash[:])
		require.NoError(err)
		exec, err := svr.dao.GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.ReadContractRequest{
			Execution:     exec.Proto().GetCore().GetExecution(),
			CallerAddress: test.callerAddr,
		}

		res, err := svr.ReadContract(context.Background(), request)
		require.NoError(err)
		require.Equal(test.retValue, res.Data)
	}
}

func TestServer_SuggestGasPrice(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range suggestGasPriceTests {
		cfg.API.GasStation.DefaultGas = test.defaultGasPrice
		svr, err := createServer(cfg, false)
		require.NoError(err)
		res, err := svr.SuggestGasPrice(context.Background(), &iotexapi.SuggestGasPriceRequest{})
		require.NoError(err)
		require.Equal(test.suggestedGasPrice, res.GasPrice)
	}
}

func TestServer_EstimateGasForAction(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range estimateGasForActionTests {
		hash, err := hash.HexStringToHash256(test.actionHash)
		require.NoError(err)
		ai, err := svr.indexer.GetActionIndex(hash[:])
		require.NoError(err)
		act, err := svr.dao.GetActionByActionHash(hash, ai.BlockHeight())
		require.NoError(err)
		request := &iotexapi.EstimateGasForActionRequest{Action: act.Proto()}

		res, err := svr.EstimateGasForAction(context.Background(), request)
		require.NoError(err)
		require.Equal(test.estimatedGas, res.Gas)
	}
}

func TestServer_EstimateActionGasConsumption(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	svr, err := createServer(cfg, false)
	require.NoError(err)

	// test for contract deploy
	data := "608060405234801561001057600080fd5b50610123600102600281600019169055503373ffffffffffffffffffffffffffffffffffffffff166001026003816000191690555060035460025417600481600019169055506102ae806100656000396000f300608060405260043610610078576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630cc0e1fb1461007d57806328f371aa146100b05780636b1d752b146100df578063d4b8399214610112578063daea85c514610145578063eb6fd96a14610188575b600080fd5b34801561008957600080fd5b506100926101bb565b60405180826000191660001916815260200191505060405180910390f35b3480156100bc57600080fd5b506100c56101c1565b604051808215151515815260200191505060405180910390f35b3480156100eb57600080fd5b506100f46101d7565b60405180826000191660001916815260200191505060405180910390f35b34801561011e57600080fd5b506101276101dd565b60405180826000191660001916815260200191505060405180910390f35b34801561015157600080fd5b50610186600480360381019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506101e3565b005b34801561019457600080fd5b5061019d61027c565b60405180826000191660001916815260200191505060405180910390f35b60035481565b6000600454600019166001546000191614905090565b60025481565b60045481565b3373ffffffffffffffffffffffffffffffffffffffff166001028173ffffffffffffffffffffffffffffffffffffffff16600102176001816000191690555060016000808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff02191690831515021790555050565b600154815600a165627a7a7230582089b5f99476d642b66a213c12cd198207b2e813bb1caf3bd75e22be535ebf5d130029"
	byteCodes, err := hex.DecodeString(data)
	require.NoError(err)
	execution, err := action.NewExecution("", 1, big.NewInt(0), 0, big.NewInt(0), byteCodes)
	require.NoError(err)
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: execution.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err := svr.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(286579), res.Gas)

	// test for transfer
	tran, err := action.NewTransfer(0, big.NewInt(0), "", []byte("123"), 0, big.NewInt(0))
	require.NoError(err)
	request = &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Transfer{
			Transfer: tran.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err = svr.EstimateActionGasConsumption(context.Background(), request)
	require.NoError(err)
	require.Equal(uint64(10300), res.Gas)
}

func TestServer_ReadUnclaimedBalance(t *testing.T) {
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	for _, test := range readUnclaimedBalanceTests {
		out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(test.addr)},
		})
		if test.returnErr {
			require.Error(t, err)
			continue
		}
		require.NoError(t, err)
		val, ok := big.NewInt(0).SetString(string(out.Data), 10)
		require.True(t, ok)
		assert.Equal(t, test.balance, val)
	}
}

func TestServer_TotalBalance(t *testing.T) {
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("TotalBalance"),
		Arguments:  nil,
	})
	require.NoError(t, err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(t, ok)
	assert.Equal(t, unit.ConvertIotxToRau(200000000), val)
}

func TestServer_AvailableBalance(t *testing.T) {
	cfg := newConfig(t)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	svr, err := createServer(cfg, false)
	require.NoError(t, err)

	out, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
		ProtocolID: []byte("rewarding"),
		MethodName: []byte("AvailableBalance"),
		Arguments:  nil,
	})
	require.NoError(t, err)
	val, ok := big.NewInt(0).SetString(string(out.Data), 10)
	require.True(t, ok)
	assert.Equal(t, unit.ConvertIotxToRau(199999936), val)
}

func TestServer_ReadCandidatesByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	committee := mock_committee.NewMockCommittee(ctrl)
	candidates := []*state.Candidate{
		{
			Address:       "address1",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
		{
			Address:       "address2",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
	}

	for _, test := range readCandidatesByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				&cfg.Genesis,
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return candidates, 0, nil
				},
				nil,
				nil,
				indexer,
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
		}
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(pol.ForceRegister(svr.registry))

		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.epoch, 10))},
		})
		require.NoError(err)
		var delegates state.CandidateList
		require.NoError(delegates.Deserialize(res.Data))
		require.Equal(test.numDelegates, len(delegates))
	}
}

func TestServer_ReadBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	committee := mock_committee.NewMockCommittee(ctrl)
	candidates := []*state.Candidate{
		{
			Address:       "address1",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
		{
			Address:       "address2",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
	}

	for _, test := range readBlockProducersByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				&cfg.Genesis,
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return candidates, 0, nil
				},
				nil,
				nil,
				indexer,
				test.numCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)

			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
		}
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(pol.ForceRegister(svr.registry))
		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.epoch, 10))},
		})
		require.NoError(err)
		var blockProducers state.CandidateList
		require.NoError(blockProducers.Deserialize(res.Data))
		require.Equal(test.numBlockProducers, len(blockProducers))
	}
}

func TestServer_ReadActiveBlockProducersByEpoch(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	committee := mock_committee.NewMockCommittee(ctrl)
	candidates := []*state.Candidate{
		{
			Address:       "address1",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
		{
			Address:       "address2",
			Votes:         big.NewInt(1),
			RewardAddress: "rewardAddress",
		},
	}

	for _, test := range readActiveBlockProducersByEpochTests {
		var pol poll.Protocol
		if test.protocolType == lld {
			cfg.Genesis.Delegates = delegates
			pol = poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
		} else {
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				&cfg.Genesis,
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return candidates, 0, nil
				},
				nil,
				nil,
				indexer,
				cfg.Genesis.NumCandidateDelegates,
				test.numDelegates,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ = poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
		}
		svr, err := createServer(cfg, false)
		require.NoError(err)
		require.NoError(pol.ForceRegister(svr.registry))

		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.epoch, 10))},
		})
		require.NoError(err)
		var activeBlockProducers state.CandidateList
		require.NoError(activeBlockProducers.Deserialize(res.Data))
		require.Equal(test.numActiveBlockProducers, len(activeBlockProducers))
	}
}

func TestServer_ReadRollDPoSMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readRollDPoSMetaTests {
		svr, err := createServer(cfg, false)
		require.NoError(err)
		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
		})
		require.NoError(err)
		result, err := strconv.ParseUint(string(res.Data), 10, 64)
		require.NoError(err)
		require.Equal(test.result, result)
	}
}

func TestServer_ReadEpochCtx(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	for _, test := range readEpochCtxTests {
		svr, err := createServer(cfg, false)
		require.NoError(err)
		res, err := svr.ReadState(context.Background(), &iotexapi.ReadStateRequest{
			ProtocolID: []byte(test.protocolID),
			MethodName: []byte(test.methodName),
			Arguments:  [][]byte{[]byte(strconv.FormatUint(test.argument, 10))},
		})
		require.NoError(err)
		result, err := strconv.ParseUint(string(res.Data), 10, 64)
		require.NoError(err)
		require.Equal(test.result, result)
	}
}

func TestServer_GetEpochMeta(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	svr, err := createServer(cfg, false)
	require.NoError(err)
	for _, test := range getEpochMetaTests {
		if test.pollProtocolType == lld {
			pol := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
			require.NoError(pol.ForceRegister(svr.registry))
		} else if test.pollProtocolType == "governanceChainCommittee" {
			committee := mock_committee.NewMockCommittee(ctrl)
			mbc := mock_blockchain.NewMockBlockchain(ctrl)
			indexer, err := poll.NewCandidateIndexer(db.NewMemKVStore())
			require.NoError(err)
			slasher, _ := poll.NewSlasher(
				&cfg.Genesis,
				func(uint64, uint64) (map[string]uint64, error) {
					return nil, nil
				},
				func(protocol.StateReader, uint64, bool, bool) ([]*state.Candidate, uint64, error) {
					return []*state.Candidate{
						{
							Address:       identityset.Address(1).String(),
							Votes:         big.NewInt(6),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(2).String(),
							Votes:         big.NewInt(5),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(3).String(),
							Votes:         big.NewInt(4),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(4).String(),
							Votes:         big.NewInt(3),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(5).String(),
							Votes:         big.NewInt(2),
							RewardAddress: "rewardAddress",
						},
						{
							Address:       identityset.Address(6).String(),
							Votes:         big.NewInt(1),
							RewardAddress: "rewardAddress",
						},
					}, 0, nil
				},
				nil,
				nil,
				indexer,
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.ProductivityThreshold,
				cfg.Genesis.ProbationEpochPeriod,
				cfg.Genesis.UnproductiveDelegateMaxCacheSize,
				cfg.Genesis.ProbationIntensityRate)
			pol, _ := poll.NewGovernanceChainCommitteeProtocol(
				indexer,
				committee,
				uint64(123456),
				func(uint64) (time.Time, error) { return time.Now(), nil },
				cfg.Chain.PollInitialCandidatesInterval,
				slasher)
			require.NoError(pol.ForceRegister(svr.registry))
			committee.EXPECT().HeightByTime(gomock.Any()).Return(test.epochData.GravityChainStartHeight, nil)

			mbc.EXPECT().TipHeight().Return(uint64(4)).Times(4)
			mbc.EXPECT().BlockHeaderByHeight(gomock.Any()).DoAndReturn(func(height uint64) (*block.Header, error) {
				if height > 0 && height <= 4 {
					pk := identityset.PrivateKey(int(height))
					blk, err := block.NewBuilder(
						block.NewRunnableActionsBuilder().Build(),
					).
						SetHeight(height).
						SetTimestamp(time.Time{}).
						SignAndBuild(pk)
					if err != nil {
						return &block.Header{}, err
					}
					return &blk.Header, nil
				}
				return &block.Header{}, errors.Errorf("invalid block height %d", height)
			}).AnyTimes()
			svr.bc = mbc
		}
		res, err := svr.GetEpochMeta(context.Background(), &iotexapi.GetEpochMetaRequest{EpochNumber: test.EpochNumber})
		require.NoError(err)
		require.Equal(test.epochData.Num, res.EpochData.Num)
		require.Equal(test.epochData.Height, res.EpochData.Height)
		require.Equal(test.epochData.GravityChainStartHeight, res.EpochData.GravityChainStartHeight)
		require.Equal(test.numBlksInEpoch, int(res.TotalBlocks))
		require.Equal(test.numConsenusBlockProducers, len(res.BlockProducersInfo))
		var numActiveBlockProducers int
		var prevInfo *iotexapi.BlockProducerInfo
		for _, bp := range res.BlockProducersInfo {
			if bp.Active {
				numActiveBlockProducers++
			}
			if prevInfo != nil {
				prevVotes, _ := strconv.Atoi(prevInfo.Votes)
				currVotes, _ := strconv.Atoi(bp.Votes)
				require.True(prevVotes >= currVotes)
			}
			prevInfo = bp
		}
		require.Equal(test.numActiveCensusBlockProducers, numActiveBlockProducers)
	}
}

func TestServer_GetRawBlocks(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getRawBlocksTest {
		request := &iotexapi.GetRawBlocksRequest{
			StartHeight:  test.startHeight,
			Count:        test.count,
			WithReceipts: test.withReceipts,
		}
		res, err := svr.GetRawBlocks(context.Background(), request)
		require.NoError(err)
		blkInfos := res.Blocks
		require.Equal(test.numBlks, len(blkInfos))
		var numActions int
		var numReceipts int
		for _, blkInfo := range blkInfos {
			numActions += len(blkInfo.Block.Body.Actions)
			numReceipts += len(blkInfo.Receipts)
		}
		require.Equal(test.numActions, numActions)
		require.Equal(test.numReceipts, numReceipts)
	}
}

func TestServer_GetLogs(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	for _, test := range getLogsTest {
		request := &iotexapi.GetLogsRequest{
			Filter: &iotexapi.LogsFilter{
				Address: test.address,
				Topics:  test.topics,
			},
			Lookup: &iotexapi.GetLogsRequest_ByRange{
				ByRange: &iotexapi.GetLogsByRange{
					FromBlock: test.fromBlock,
					Count:     test.count,
				},
			},
		}
		res, err := svr.GetLogs(context.Background(), request)
		require.NoError(err)
		logs := res.Logs
		require.Equal(test.numLogs, len(logs))
	}
}

func TestServer_GetEvmTransfersByActionHash(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	request := &iotexapi.GetEvmTransfersByActionHashRequest{
		ActionHash: hex.EncodeToString(hash.ZeroHash256[:]),
	}
	_, err = svr.GetEvmTransfersByActionHash(context.Background(), request)
	require.Error(err)
	sta, ok := status.FromError(err)
	require.Equal(true, ok)
	require.Equal(codes.NotFound, sta.Code())

	for _, test := range getEvmTransfersByActionHashTest {
		request := &iotexapi.GetEvmTransfersByActionHashRequest{
			ActionHash: hex.EncodeToString(test.actHash[:]),
		}
		res, err := svr.GetEvmTransfersByActionHash(context.Background(), request)
		require.NoError(err)

		transfers := res.ActionEvmTransfers
		require.Equal(test.numEvmTransfer, transfers.NumEvmTransfers)
		require.Equal(test.numEvmTransfer, uint64(len(transfers.EvmTransfers)))
		require.Equal(test.actHash[:], transfers.ActionHash)
		for i := 0; i < len(transfers.EvmTransfers); i++ {
			require.Equal(test.amount[i], transfers.EvmTransfers[i].Amount)
			require.Equal(test.from[i], transfers.EvmTransfers[i].From)
			require.Equal(test.to[i], transfers.EvmTransfers[i].To)
		}
	}
}
func TestServer_GetEvmTransfersByBlockHeight(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)

	svr, err := createServer(cfg, false)
	require.NoError(err)

	request := &iotexapi.GetEvmTransfersByBlockHeightRequest{
		BlockHeight: 101,
	}
	_, err = svr.GetEvmTransfersByBlockHeight(context.Background(), request)
	require.Error(err)
	sta, ok := status.FromError(err)
	require.Equal(true, ok)
	require.Equal(codes.InvalidArgument, sta.Code())

	request.BlockHeight = 2
	_, err = svr.GetEvmTransfersByBlockHeight(context.Background(), request)
	require.Error(err)
	sta, ok = status.FromError(err)
	require.Equal(true, ok)
	require.Equal(codes.NotFound, sta.Code())

	for _, test := range getEvmTransfersByBlockHeightTest {
		request := &iotexapi.GetEvmTransfersByBlockHeightRequest{
			BlockHeight: test.height,
		}
		res, err := svr.GetEvmTransfersByBlockHeight(context.Background(), request)
		require.NoError(err)

		transfers := res.BlockEvmTransfers
		require.Equal(test.numEvmTransfer, transfers.NumEvmTransfers)
		require.Equal(test.numEvmTransfer, uint64(len(transfers.ActionEvmTransfers)))
		require.Equal(test.height, transfers.BlockHeight)
		for i := 0; i < len(transfers.ActionEvmTransfers); i++ {
			require.Equal(test.actTransfers[i].actHash[:], transfers.ActionEvmTransfers[i].ActionHash)
			for j := 0; j < len(transfers.ActionEvmTransfers[i].EvmTransfers); j++ {
				require.Equal(test.actTransfers[i].amount[j], transfers.ActionEvmTransfers[i].EvmTransfers[j].Amount)
				require.Equal(test.actTransfers[i].from[j], transfers.ActionEvmTransfers[i].EvmTransfers[j].From)
				require.Equal(test.actTransfers[i].to[j], transfers.ActionEvmTransfers[i].EvmTransfers[j].To)
			}
		}
	}
}

func addTestingBlocks(bc blockchain.Blockchain) error {
	addr0 := identityset.Address(27).String()
	priKey0 := identityset.PrivateKey(27)
	addr1 := identityset.Address(28).String()
	priKey1 := identityset.PrivateKey(28)
	addr2 := identityset.Address(29).String()
	addr3 := identityset.Address(30).String()
	priKey3 := identityset.PrivateKey(30)
	addr4 := identityset.Address(31).String()
	// Add block 1
	// Producer transfer--> C
	tsf, err := testutil.SignedTransfer(addr3, priKey0, 1, big.NewInt(10), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}

	blk1Time := testutil.TimestampNow()
	actionMap := make(map[string][]action.SealedEnvelope)
	actionMap[addr0] = []action.SealedEnvelope{tsf}
	blk, err := bc.MintNewBlock(
		actionMap,
		blk1Time,
	)
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 2
	// Charlie transfer--> A, B, D, P
	// Charlie transfer--> C
	// Charlie exec--> D
	recipients := []string{addr1, addr2, addr4, addr0}
	selps := make([]action.SealedEnvelope, 0)
	for i, recipient := range recipients {
		selp, err := testutil.SignedTransfer(recipient, priKey3, uint64(i+1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
		if err != nil {
			return err
		}
		selps = append(selps, selp)
	}
	selp, err := testutil.SignedTransfer(addr3, priKey3, uint64(5), big.NewInt(2), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	execution1, err := testutil.SignedExecution(addr4, priKey3, 6,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	selps = append(selps, selp)
	selps = append(selps, execution1)
	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = selps
	if blk, err = bc.MintNewBlock(
		actionMap,
		blk1Time.Add(time.Second),
	); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 3
	// Empty actions
	if blk, err = bc.MintNewBlock(
		nil,
		blk1Time.Add(time.Second*2),
	); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}

	// Add block 4
	// Charlie transfer--> C
	// Alfa transfer--> A
	// Charlie exec--> D
	// Alfa exec--> D
	tsf1, err := testutil.SignedTransfer(addr3, priKey3, uint64(7), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	tsf2, err := testutil.SignedTransfer(addr1, priKey1, uint64(1), big.NewInt(1), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	execution1, err = testutil.SignedExecution(addr4, priKey3, 8,
		big.NewInt(2), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}
	execution2, err := testutil.SignedExecution(addr4, priKey1, 2,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64), []byte{1})
	if err != nil {
		return err
	}

	actionMap = make(map[string][]action.SealedEnvelope)
	actionMap[addr3] = []action.SealedEnvelope{tsf1, execution1}
	actionMap[addr1] = []action.SealedEnvelope{tsf2, execution2}
	if blk, err = bc.MintNewBlock(
		actionMap,
		blk1Time.Add(time.Second*3),
	); err != nil {
		return err
	}
	return bc.CommitBlock(blk)
}

func addActsToActPool(ctx context.Context, ap actpool.ActPool) error {
	// Producer transfer--> A
	tsf1, err := testutil.SignedTransfer(identityset.Address(28).String(), identityset.PrivateKey(27), 2, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> P
	tsf2, err := testutil.SignedTransfer(identityset.Address(27).String(), identityset.PrivateKey(27), 3, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer transfer--> B
	tsf3, err := testutil.SignedTransfer(identityset.Address(29).String(), identityset.PrivateKey(27), 4, big.NewInt(20), []byte{}, testutil.TestGasLimit, big.NewInt(testutil.TestGasPriceInt64))
	if err != nil {
		return err
	}
	// Producer exec--> D
	execution1, err := testutil.SignedExecution(identityset.Address(31).String(), identityset.PrivateKey(27), 5,
		big.NewInt(1), testutil.TestGasLimit, big.NewInt(10), []byte{1})
	if err != nil {
		return err
	}

	if err := ap.Add(ctx, tsf1); err != nil {
		return err
	}
	if err := ap.Add(ctx, tsf2); err != nil {
		return err
	}
	if err := ap.Add(ctx, tsf3); err != nil {
		return err
	}
	return ap.Add(ctx, execution1)
}

func setupChain(cfg config.Config) (blockchain.Blockchain, blockdao.BlockDAO, blockindex.Indexer, *systemlog.Indexer, factory.Factory, *protocol.Registry, error) {
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	registry := protocol.NewRegistry()
	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption(), factory.RegistryOption(registry))
	if err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	cfg.Genesis.InitBalanceMap[identityset.Address(27).String()] = unit.ConvertIotxToRau(10000000000).String()
	// create indexer
	indexer, err := blockindex.NewIndexer(db.NewMemKVStore(), cfg.Genesis.Hash())
	if err != nil {
		return nil, nil, nil, nil, nil, nil, errors.New("failed to create indexer")
	}
	systemLogIndexer, err := systemlog.NewIndexer(db.NewMemKVStore())
	if err != nil {
		return nil, nil, nil, nil, nil, nil, errors.New("failed to create systemlog indexer")
	}
	// create BlockDAO
	// systemLogIndexer is not added into blockDao
	dao := blockdao.NewBlockDAO(db.NewMemKVStore(), []blockdao.BlockIndexer{sf, indexer}, cfg.Chain.CompressBlock, cfg.DB)
	if dao == nil {
		return nil, nil, nil, nil, nil, nil, errors.New("failed to create blockdao")
	}
	// create chain
	bc := blockchain.NewBlockchain(
		cfg,
		dao,
		sf,
		blockchain.BlockValidatorOption(block.NewValidator(
			sf,
			protocol.NewGenericValidator(sf, accountutil.AccountState),
		)),
	)
	if bc == nil {
		return nil, nil, nil, nil, nil, nil, errors.New("failed to create blockchain")
	}
	defer func() {
		delete(cfg.Plugins, config.GatewayPlugin)
	}()

	acc := account.NewProtocol(rewarding.DepositGas)
	evm := execution.NewProtocol(dao.GetBlockHash, rewarding.DepositGas)
	p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
	rolldposProtocol := rolldpos.NewProtocol(
		genesis.Default.NumCandidateDelegates,
		genesis.Default.NumDelegates,
		genesis.Default.NumSubEpochs,
		rolldpos.EnableDardanellesSubEpoch(cfg.Genesis.DardanellesBlockHeight, cfg.Genesis.DardanellesNumSubEpochs),
	)
	r := rewarding.NewProtocol(
		func(uint64, uint64) (map[string]uint64, error) {
			return nil, nil
		})

	if err := rolldposProtocol.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := acc.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := evm.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := r.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}
	if err := p.Register(registry); err != nil {
		return nil, nil, nil, nil, nil, nil, err
	}

	return bc, dao, indexer, systemLogIndexer, sf, registry, nil
}

func setupActPool(sf factory.Factory, cfg config.ActPool) (actpool.ActPool, error) {
	ap, err := actpool.NewActPool(sf, cfg, actpool.EnableExperimentalActions())
	if err != nil {
		return nil, err
	}

	ap.AddActionEnvelopeValidators(protocol.NewGenericValidator(sf, accountutil.AccountState))

	return ap, nil
}

func setupSystemLogIndexer(indexer *systemlog.Indexer) error {
	blk, err := block.NewTestingBuilder().
		SetHeight(1).
		SignAndBuild(identityset.PrivateKey(30))
	if err != nil {
		return err
	}
	emptyBlock, err := block.NewTestingBuilder().
		SetHeight(2).
		SignAndBuild(identityset.PrivateKey(30))
	if err != nil {
		return err
	}
	blk.Receipts = []*action.Receipt{testReceiptWithSystemLog}

	ctx := context.Background()
	if err := indexer.Start(ctx); err != nil {
		return err
	}

	if err := indexer.PutBlock(ctx, &blk); err != nil {
		return err
	}
	if err := indexer.PutBlock(ctx, &emptyBlock); err != nil {
		return err
	}

	return nil
}

func newConfig(t *testing.T) config.Config {
	r := require.New(t)
	cfg := config.Default

	testTriePath, err := testutil.PathOfTempFile("trie")
	r.NoError(err)
	testDBPath, err := testutil.PathOfTempFile("db")
	r.NoError(err)
	testIndexPath, err := testutil.PathOfTempFile("index")
	r.NoError(err)
	testSystemLogPath, err := testutil.PathOfTempFile("systemlog")
	r.NoError(err)

	cfg.Plugins[config.GatewayPlugin] = true
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = testIndexPath
	cfg.System.SystemLogDBPath = testSystemLogPath
	cfg.Chain.EnableAsyncIndexWrite = false
	cfg.Genesis.EnableGravityChainVoting = true
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.API.RangeQueryLimit = 100

	return cfg
}

func createServer(cfg config.Config, needActPool bool) (*Server, error) {
	// TODO (zhi): revise
	bc, dao, indexer, systemLogIndexer, sf, registry, err := setupChain(cfg)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	// Start blockchain
	if err := bc.Start(ctx); err != nil {
		return nil, err
	}

	// Add testing blocks
	if err := addTestingBlocks(bc); err != nil {
		return nil, err
	}

	// setup system log indexer
	if err := setupSystemLogIndexer(systemLogIndexer); err != nil {
		return nil, err
	}

	var ap actpool.ActPool
	if needActPool {
		ap, err = setupActPool(sf, cfg.ActPool)
		if err != nil {
			return nil, err
		}
		// Add actions to actpool
		ctx := protocol.WithRegistry(context.Background(), registry)
		if err := addActsToActPool(ctx, ap); err != nil {
			return nil, err
		}
	}

	svr := &Server{
		bc:               bc,
		sf:               sf,
		dao:              dao,
		indexer:          indexer,
		systemLogIndexer: systemLogIndexer,
		ap:               ap,
		cfg:              cfg,
		gs:               gasstation.NewGasStation(bc, sf.SimulateExecution, dao, cfg.API),
		registry:         registry,
		hasActionIndex:   true,
	}

	return svr, nil
}

func TestServer_EstimateActionGasConsumption2(t *testing.T) {
	require := require.New(t)
	cfg := newConfig(t)
	svr, err := createServer(cfg, false)
	require.NoError(err)

	// test for contract deploy
	data := "0x60806040523480156200001157600080fd5b506040516200490838038062004908833981018060405260608110156200003757600080fd5b8101908080516401000000008111156200005057600080fd5b828101905060208101848111156200006757600080fd5b81518560208202830111640100000000821117156200008557600080fd5b50509291906020018051640100000000811115620000a257600080fd5b82810190506020810184811115620000b957600080fd5b8151856020820283011164010000000082111715620000d757600080fd5b50509291906020018051640100000000811115620000f457600080fd5b828101905060208101848111156200010b57600080fd5b81518560208202830111640100000000821117156200012957600080fd5b505092919050505060008351111515620001ab576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601d8152602001807f6f726773206c656e677468206d7573742067726561746572207a65726f00000081525060200191505060405180910390fd5b815183511415156200024b576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260358152602001807f6c656e677468206f66206f72677320697320646966666572656e742066726f6d81526020017f206c656e677468206f662063617061636974696573000000000000000000000081525060400191505060405180910390fd5b600080905060008090505b84518160ff16101562000328576000848260ff168151811015156200027757fe5b9060200190602002015160ff16111515620002fa576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601e8152602001807f6f7267206361706163697479206d7573742067726561746572207a65726f000081525060200191505060405180910390fd5b838160ff168151811015156200030c57fe5b9060200190602002015182019150808060010191505062000256565b5081518160ff16141515620003cb576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252603d8152602001807f6c656e677468206f662064656c65676174657320697320646966666572656e7481526020017f2066726f6d20746f74616c206f66206f7267206361706163697469657300000081525060400191505060405180910390fd5b600080905060008090505b85518160ff16101562000760576004868260ff16815181101515620003f757fe5b9060200190602002015190806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050600160056000888460ff168151811015156200047c57fe5b9060200190602002015173ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160006101000a81548160ff021916908315150217905550848160ff16815181101515620004ec57fe5b9060200190602002015160056000888460ff168151811015156200050c57fe5b9060200190602002015173ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160016101000a81548160ff021916908360ff16021790555060008090505b858260ff168151811015156200058357fe5b9060200190602002015160ff168160ff161015620007515760056000888460ff16815181101515620005b157fe5b9060200190602002015173ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101858460ff168151811015156200060a57fe5b9060200190602002015190806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff160217905550506040805190810160405280600115158152602001602060405190810160405280600081525081525060066000878660ff16815181101515620006b557fe5b9060200190602002015173ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201518160000160006101000a81548160ff0219169083151502179055506020820151816001019080519060200190620007379291906200076c565b509050508280600101935050808060010191505062000571565b508080600101915050620003d6565b5050505050506200081b565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10620007af57805160ff1916838001178555620007e0565b82800160010185558215620007e0579182015b82811115620007df578251825591602001919060010190620007c2565b5b509050620007ef9190620007f3565b5090565b6200081891905b8082111562000814576000816000905550600101620007fa565b5090565b90565b6140dd806200082b6000396000f3fe6080604052600436106100d0576000357c0100000000000000000000000000000000000000000000000000000000900463ffffffff1680630f6d17f9146100d557806313e7c9d8146101335780632430df89146101d9578063555ed8bb146102c15780636138b19e14610312578063753ec1031461037e5780638008df4f146104c1578063a03eb838146104fc578063a230c5241461055a578063ac8a584a146105c3578063b44d5fc714610614578063b759f954146106de578063bdd4d18d14610719578063e1d1f05314610785575b600080fd5b3480156100e157600080fd5b50610131600480360360408110156100f857600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803560ff16906020019092919050505061086d565b005b34801561013f57600080fd5b506101826004803603602081101561015657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050610bf0565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b838110156101c55780820151818401526020810190506101aa565b505050509050019250505060405180910390f35b3480156101e557600080fd5b506102bf600480360360408110156101fc57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291908035906020019064010000000081111561023957600080fd5b82018360208201111561024b57600080fd5b8035906020019184600183028401116401000000008311171561026d57600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f820116905080830192505050505050509192919290505050610cc0565b005b3480156102cd57600080fd5b50610310600480360360208110156102e457600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061117b565b005b34801561031e57600080fd5b506103276114dc565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b8381101561036a57808201518184015260208101905061034f565b505050509050019250505060405180910390f35b34801561038a57600080fd5b50610393611797565b604051808881526020018773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018660038111156103db57fe5b60ff1681526020018573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018460ff1660ff1681526020018060200180602001838103835285818151815260200191508051906020019060200280838360005b83811015610464578082015181840152602081019050610449565b50505050905001838103825284818151815260200191508051906020019060200280838360005b838110156104a657808201518184015260208101905061048b565b50505050905001995050505050505050505060405180910390f35b3480156104cd57600080fd5b506104fa600480360360208110156104e457600080fd5b8101908080359060200190929190505050611d36565b005b34801561050857600080fd5b506105586004803603604081101561051f57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803560ff169060200190929190505050611d44565b005b34801561056657600080fd5b506105a96004803603602081101561057d57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190505050612156565b604051808215151515815260200191505060405180910390f35b3480156105cf57600080fd5b50610612600480360360208110156105e657600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506121af565b005b34801561062057600080fd5b506106636004803603602081101561063757600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506126b9565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156106a3578082015181840152602081019050610688565b50505050905090810190601f1680156106d05780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b3480156106ea57600080fd5b506107176004803603602081101561070157600080fd5b8101908080359060200190929190505050612861565b005b34801561072557600080fd5b5061072e61286f565b6040518080602001828103825283818151815260200191508051906020019060200280838360005b83811015610771578082015181840152602081019050610756565b505050509050019250505060405180910390f35b34801561079157600080fd5b5061086b600480360360408110156107a857600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803590602001906401000000008111156107e557600080fd5b8201836020820111156107f757600080fd5b8035906020019184600183028401116401000000008311171561081957600080fd5b91908080601f016020809104026020016040519081016040528093929190818152602001838380828437600081840152601f19601f8201169050808301925050505050505091929192905050506128fd565b005b61087633612156565b1515610910576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f6f6e6c79206d656d6265722063616e2063616c6c20746869732066756e63746981526020017f6f6e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff16600060010160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415156109d9576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260168152602001807f616c72656164792065786973742070726f706f73616c0000000000000000000081525060200191505060405180910390fd5b6109e282612156565b151515610a57576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260148152602001807f6d656d62657220616c726561647920657869737400000000000000000000000081525060200191505060405180910390fd5b33600060010160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506001600060010160146101000a81548160ff02191690836003811115610abd57fe5b021790555081600060020160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600060020160146101000a81548160ff021916908360ff1602179055507f8665d2ae7d62e4273cdfc40af1cbbea2232b315ca2a2e244c833366f2b8ae9eb60008001543360018585604051808681526020018573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001846003811115610b9757fe5b60ff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018260ff1660ff1681526020019550505050505060405180910390a15050565b6060600560008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101805480602002602001604051908101604052809291908181526020018280548015610cb457602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019060010190808311610c6a575b50505050509050919050565b610cc933612156565b1515610d63576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f6f6e6c79206d656d6265722063616e2063616c6c20746869732066756e63746981526020017f6f6e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b600660008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160009054906101000a900460ff16151515610e28576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601f8152602001807f6f70657261746f72206164647265737320616c7265616479206578697374730081525060200191505060405180910390fd5b600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160019054906101000a900460ff1660ff16600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010180549050101515610f36576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260118152602001807f6f70657261746f72732069732066756c6c00000000000000000000000000000081525060200191505060405180910390fd5b600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018290806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050604080519081016040528060011515815260200182815250600660008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201518160000160006101000a81548160ff021916908315150217905550602082015181600101908051906020019061106f929190613e72565b509050507fc1ad0c93e1873e80f0551582678c47e2e3cd93d9f12f18b1012ed1ad51654d74338383604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200180602001828103825283818151815260200191508051906020019080838360005b8381101561113b578082015181840152602081019050611120565b50505050905090810190601f1680156111685780820380516001836020036101000a031916815260200191505b5094505050505060405180910390a15050565b61118433612156565b151561121e576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f6f6e6c79206d656d6265722063616e2063616c6c20746869732066756e63746981526020017f6f6e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff16600060010160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415156112e7576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260168152602001807f616c72656164792065786973742070726f706f73616c0000000000000000000081525060200191505060405180910390fd5b6112f081612156565b1515611364576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260158152602001807f6d656d62657220646f6573206e6f74206578697374000000000000000000000081525060200191505060405180910390fd5b33600060010160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506003600060010160146101000a81548160ff021916908360038111156113ca57fe5b021790555080600060020160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055507f8665d2ae7d62e4273cdfc40af1cbbea2232b315ca2a2e244c833366f2b8ae9eb6000800154336003846000604051808681526020018573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200184600381111561148757fe5b60ff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018260ff1681526020019550505050505060405180910390a150565b6060600080905060008090505b6004805490508160ff16101561158c576005600060048360ff1681548110151561150f57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101805490508201915080806001019150506114e9565b506060816040519080825280602002602001820160405280156115be5781602001602082028038833980820191505090505b50905060008090505b6004805490508160ff16101561178e5760008090505b6005600060048460ff168154811015156115f357fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101805490508160ff161015611780576005600060048460ff1681548110151561167f57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018160ff168154811015156116f757fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1683856001900395508581518110151561173757fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff168152505080806001019150506115dd565b5080806001019150506115c7565b50809250505090565b6000806000806000606080600073ffffffffffffffffffffffffffffffffffffffff16600060010160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16141561181657868686868686869650965096509650965096509650611d2d565b60008001549650600060010160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169550600060010160149054906101000a900460ff169450600060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff169350600060020160149054906101000a900460ff1692506000809050600080905060008090505b600480549050811015611a2557600160028111156118c157fe5b600060030160006004848154811015156118d757fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff16600281111561195557fe5b1415611968578280600101935050611a18565b60028081111561197457fe5b6000600301600060048481548110151561198a57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff166002811115611a0857fe5b1415611a175781806001019250505b5b80806001019150506118a7565b5081604051908082528060200260200182016040528015611a555781602001602082028038833980820191505090505b50935080604051908082528060200260200182016040528015611a875781602001602082028038833980820191505090505b50925060008090505b600480549050811015611d145760016002811115611aaa57fe5b60006003016000600484815481101515611ac057fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff166002811115611b3e57fe5b1415611bd457600481815481101515611b5357fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16858460019003945084815181101515611b9357fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff1681525050611d07565b600280811115611be057fe5b60006003016000600484815481101515611bf657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff166002811115611c7457fe5b1415611d0657600481815481101515611c8957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16848360019003935083815181101515611cc957fe5b9060200190602002019073ffffffffffffffffffffffffffffffffffffffff16908173ffffffffffffffffffffffffffffffffffffffff16815250505b5b8080600101915050611a90565b5088888888888888985098509850985098509850985050505b90919293949596565b611d41816000612c90565b50565b611d4d33612156565b1515611de7576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f6f6e6c79206d656d6265722063616e2063616c6c20746869732066756e63746981526020017f6f6e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff16600060010160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16141515611eb0576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260168152602001807f616c72656164792065786973742070726f706f73616c0000000000000000000081525060200191505060405180910390fd5b611eb982612156565b1515611f2d576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260158152602001807f6d656d62657220646f6573206e6f74206578697374000000000000000000000081525060200191505060405180910390fd5b8060ff16600060020160149054906101000a900460ff1660ff1614151515611fbd576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260148152602001807f6361706163697479206973206964656e7469616c00000000000000000000000081525060200191505060405180910390fd5b33600060010160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055506002600060010160146101000a81548160ff0219169083600381111561202357fe5b021790555081600060020160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555080600060020160146101000a81548160ff021916908360ff1602179055507f8665d2ae7d62e4273cdfc40af1cbbea2232b315ca2a2e244c833366f2b8ae9eb60008001543360028585604051808681526020018573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018460038111156120fd57fe5b60ff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018260ff1660ff1681526020019550505050505060405180910390a15050565b6000600560008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160009054906101000a900460ff169050919050565b6121b833612156565b1515612252576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f6f6e6c79206d656d6265722063616e2063616c6c20746869732066756e63746981526020017f6f6e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b60008090505b600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101805490508160ff16101561263e578173ffffffffffffffffffffffffffffffffffffffff16600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018260ff1681548110151561231157fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415612631576001600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010180549050038160ff16101561250857600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001016001600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101805490500381548110151561244257fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018260ff168154811015156124bf57fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505b600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001016001600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101805490500381548110151561259f57fe5b9060005260206000200160006101000a81549073ffffffffffffffffffffffffffffffffffffffff0219169055600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018054809190600190036126219190613ef2565b5061262b82613ad0565b506126b6565b8080600101915050612258565b50600015156126b5576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260198152602001807f6661696c656420746f2064656c657465206f70657261746f720000000000000081525060200191505060405180910390fd5b5b50565b6060600660008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160009054906101000a900460ff16151561277f576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260178152602001807f6f70657261746f7220646f6573206e6f7420657869737400000000000000000081525060200191505060405180910390fd5b600660008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156128555780601f1061282a57610100808354040283529160200191612855565b820191906000526020600020905b81548152906001019060200180831161283857829003601f168201915b50505050509050919050565b61286c816001612c90565b50565b606060048054806020026020016040519081016040528092919081815260200182805480156128f357602002820191906000526020600020905b8160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190600101908083116128a9575b5050505050905090565b61290633612156565b15156129a0576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f6f6e6c79206d656d6265722063616e2063616c6c20746869732066756e63746981526020017f6f6e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b60008090505b600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101805490508160ff161015612c14578273ffffffffffffffffffffffffffffffffffffffff16600560003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018260ff16815481101515612a5f57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415612c075781600660008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001019080519060200190612afc929190613f1e565b507f8daf704ffb23b2e479e9849d1bda8860345f6c15d43b68cd9cf8622952564895338484604051808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200180602001828103825283818151815260200191508051906020019080838360005b83811015612bc5578082015181840152602081019050612baa565b50505050905090810190601f168015612bf25780820380516001836020036101000a031916815260200191505b5094505050505060405180910390a150612c8c565b80806001019150506129a6565b5060001515612c8b576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260178152602001807f6f70657261746f7220646f6573206e6f7420657869737400000000000000000081525060200191505060405180910390fd5b5b5050565b612c9933612156565b1515612d33576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260228152602001807f6f6e6c79206d656d6265722063616e2063616c6c20746869732066756e63746981526020017f6f6e00000000000000000000000000000000000000000000000000000000000081525060400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff16600060010160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1614151515612dfd576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260178152602001807f70726f706f73616c20646f6573206e6f7420657869737400000000000000000081525060200191505060405180910390fd5b816000800154141515612e78576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260148152602001807f70726f706f73616c20697320696e61637469766500000000000000000000000081525060200191505060405180910390fd5b8015612ee7576001600060030160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff02191690836002811115612edd57fe5b0217905550612f4c565b6002600060030160003373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81548160ff02191690836002811115612f4657fe5b02179055505b7fcfa82ef0390c8f3e57ebe6c0665352a383667e792af012d350d9786ee5173d26823383604051808481526020018373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200182151515158152602001935050505060405180910390a16000809050600080905060008090505b6004805490508160ff16101561315a5760016002811115612ff057fe5b6000600301600060048460ff1681548110151561300957fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff16600281111561308757fe5b141561309a57828060010193505061314d565b6002808111156130a657fe5b6000600301600060048460ff168154811015156130bf57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060009054906101000a900460ff16600281111561313d57fe5b141561314c5781806001019250505b5b8080600101915050612fd3565b506004805490506003820210151561317d576131766000613c2c565b5050613acc565b60026004805490500260038302111515613198575050613acc565b600160038111156131a557fe5b600060010160149054906101000a900460ff1660038111156131c357fe5b141561338d576004600060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1690806001815401808255809150509060018203906000526020600020016000909192909190916101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050606060405190810160405280600115158152602001600060020160149054906101000a900460ff1660ff16815260200160006040519080825280602002602001820160405280156132b45781602001602082028038833980820191505090505b50815250600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008201518160000160006101000a81548160ff02191690831515021790555060208201518160000160016101000a81548160ff021916908360ff1602179055506040820151816001019080519060200190613378929190613f9e565b509050506133866001613c2c565b5050613acc565b6002600381111561339a57fe5b600060010160149054906101000a900460ff1660038111156133b857fe5b14156136c1576000600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018054905090505b600060020160149054906101000a900460ff1660ff1681111561361e576134f3600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600101600183038154811015156134c357fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16613ad0565b600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001016001820381548110151561356957fe5b9060005260206000200160006101000a81549073ffffffffffffffffffffffffffffffffffffffff0219169055600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060010180548091906001900361360f9190613ef2565b5080806001900391505061342d565b50600060020160149054906101000a900460ff16600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160016101000a81548160ff021916908360ff1602179055506136ba6001613c2c565b5050613acc565b60008090505b600480549050811015613a5257600060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1660048281548110151561371f57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff161415613a455760008090505b600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018054905081101561389057613883600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001018281548110151561385357fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff16613ad0565b808060010191505061376c565b50600560008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020600080820160006101000a81549060ff02191690556000820160016101000a81549060ff021916905560018201600061392d9190614028565b50506001600480549050038110156139d957600460016004805490500381548110151561395657fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1660048281548110151561399057fe5b9060005260206000200160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff1602179055505b60046001600480549050038154811015156139f057fe5b9060005260206000200160006101000a81549073ffffffffffffffffffffffffffffffffffffffff02191690556004805480919060019003613a329190613ef2565b50613a3d6001613c2c565b505050613acc565b80806001019150506136c7565b5060001515613ac9576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825260128152602001807f63616e6e6f742066696e64206d656d626572000000000000000000000000000081525060200191505060405180910390fd5b50505b5050565b6000600660008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060000160006101000a81548160ff0219169083151502179055506020604051908101604052806000815250600660008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020016000206001019080519060200190613b91929190613f1e565b507fb157cf3e9ae29eb366b3bdda54b41d4738ada5daa73f8d2f1bef6280bb1418e43382604051808373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020018273ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019250505060405180910390a150565b600060030160008060020160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81549060ff021916905560008090505b600480549050811015613d4d5760006003016000600483815481101515613ccb57fe5b9060005260206000200160009054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006101000a81549060ff02191690558080600101915050613ca8565b5060008060010160146101000a81548160ff02191690836003811115613d6f57fe5b021790555060008060010160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060008060020160006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555060008060020160146101000a81548160ff021916908360ff1602179055507ff412ee5dcd836b3a4fa40ae9d7e90eeea743a32f4e0ba26c91c7cbc8c3d8b44b600080016000815480929190600101919050558260405180838152602001821515151581526020019250505060405180910390a150565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10613eb357805160ff1916838001178555613ee1565b82800160010185558215613ee1579182015b82811115613ee0578251825591602001919060010190613ec5565b5b509050613eee9190614049565b5090565b815481835581811115613f1957818360005260206000209182019101613f189190614049565b5b505050565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f10613f5f57805160ff1916838001178555613f8d565b82800160010185558215613f8d579182015b82811115613f8c578251825591602001919060010190613f71565b5b509050613f9a9190614049565b5090565b828054828255906000526020600020908101928215614017579160200282015b828111156140165782518260006101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555091602001919060010190613fbe565b5b509050614024919061406e565b5090565b50805460008255906000526020600020908101906140469190614049565b50565b61406b91905b8082111561406757600081600090555060010161404f565b5090565b90565b6140ae91905b808211156140aa57600081816101000a81549073ffffffffffffffffffffffffffffffffffffffff021916905550600101614074565b5090565b9056fea165627a7a723058209a09a7e6a7dddfd73f79034d0f1b2b7d94c5f09544d5bbd8bf941e332bf917160029"
	byteCodes, err := hex.DecodeString(data)
	require.NoError(err)
	execution, err := action.NewExecution("", 1, big.NewInt(0), 0, big.NewInt(0), byteCodes)
	require.NoError(err)
	request := &iotexapi.EstimateActionGasConsumptionRequest{
		Action: &iotexapi.EstimateActionGasConsumptionRequest_Execution{
			Execution: execution.Proto(),
		},
		CallerAddress: identityset.Address(0).String(),
	}
	res, err := svr.EstimateActionGasConsumption(context.Background(), request)
	fmt.Println(err)
	fmt.Println(res.String())
}
