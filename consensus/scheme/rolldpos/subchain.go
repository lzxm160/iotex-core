// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"math/big"

	"github.com/iotexproject/iotex-core/protogen/iotexapi"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/pkg/keypair"
	"github.com/iotexproject/iotex-core/pkg/log"
)

func putBlockToParentChain(
	rootChainAPI iotexapi.APIServiceClient,
	subChainAddr string,
	senderPrvKey keypair.PrivateKey,
	senderAddr string,
	b *block.Block,
) {
	if err := putBlockToParentChainTask(rootChainAPI, subChainAddr, senderPrvKey, b); err != nil {
		log.L().Error("Failed to put block merkle roots to parent chain.",
			zap.String("subChainAddress", subChainAddr),
			zap.String("senderAddress", senderAddr),
			zap.Uint64("height", b.Height()),
			zap.Error(err))
		return
	}
	log.L().Info("Succeeded to put block merkle roots to parent chain.",
		zap.String("subChainAddress", subChainAddr),
		zap.String("senderAddress", senderAddr),
		zap.Uint64("height", b.Height()))
}

func putBlockToParentChainTask(
	rootChainAPI iotexapi.APIServiceClient,
	subChainAddr string,
	senderPrvKey keypair.PrivateKey,
	b *block.Block,
) error {
	req, err := constructPutSubChainBlockRequest(rootChainAPI, subChainAddr, senderPrvKey.PublicKey(), senderPrvKey, b)
	if err != nil {
		return errors.Wrap(err, "fail to construct PutSubChainBlockRequest")
	}

	if _, err := rootChainAPI.SendAction(context.Background(), req); err != nil {
		return errors.Wrap(err, "fail to call explorerapi to put block")
	}
	return nil
}

func constructPutSubChainBlockRequest(
	rootChainAPI iotexapi.APIServiceClient,
	subChainAddr string,
	senderPubKey keypair.PublicKey,
	senderPriKey keypair.PrivateKey,
	b *block.Block,
) (*iotexapi.SendActionRequest, error) {
	senderPCAddr, err := address.FromBytes(senderPubKey.Hash())
	if err != nil {
		return nil, err
	}
	encodedSenderPCAddr := senderPCAddr.String()

	// get sender current pending nonce on parent chain
	req := &iotexapi.GetAccountRequest{Address: encodedSenderPCAddr}
	senderPCAddrDetails, err := rootChainAPI.GetAccount(context.Background(), req)
	if err != nil {
		return nil, errors.Wrap(err, "fail to get address details")
	}

	rootm := make(map[string]hash.Hash256)
	rootm["tx"] = b.TxRoot()
	pb := action.NewPutBlock(
		uint64(senderPCAddrDetails.AccountMeta.PendingNonce),
		subChainAddr,
		b.Height(),
		rootm,
		1000000,        // gas limit
		big.NewInt(10), //gas price
	)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(uint64(senderPCAddrDetails.AccountMeta.PendingNonce)).
		SetGasPrice(big.NewInt(10)).
		SetGasLimit(1000000).
		SetAction(pb).Build()

	// sign action
	selp, err := action.Sign(elp, senderPriKey)
	if err != nil {
		return nil, errors.Wrap(err, "fail to sign put block action")
	}
	request := &iotexapi.SendActionRequest{Action: selp.Proto()}
	return request, nil
}
