// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"encoding/hex"
	"math/big"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-core/action"
)

// SignedTransfer return a signed transfer
func SignedTransfer(recipientAddr string, senderPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, payload []byte, gasLimit uint64, gasPrice *big.Int) (action.SealedEnvelope, error) {
	transfer, err := action.NewTransfer(nonce, amount, recipientAddr, payload, gasLimit, gasPrice)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(transfer).Build()
	selp, err := action.Sign(elp, senderPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign transfer %v", elp)
	}
	return selp, nil
}

// SignedExecution return a signed execution
func SignedExecution(contractAddr string, executorPriKey crypto.PrivateKey, nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) (action.SealedEnvelope, error) {
	execution, err := action.NewExecution(contractAddr, nonce, amount, gasLimit, gasPrice, data)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetNonce(nonce).
		SetGasPrice(gasPrice).
		SetGasLimit(gasLimit).
		SetAction(execution).Build()
	selp, err := action.Sign(elp, executorPriKey)
	if err != nil {
		return action.SealedEnvelope{}, errors.Wrapf(err, "failed to sign execution %v", elp)
	}
	return selp, nil
}
func CreateBlockchain(inMem bool, cfg config.Config, protocols []string) (bc blockchain.Blockchain, dao blockdao.BlockDAO, indexer blockindex.Indexer, registry *protocol.Registry, sf factory.Factory, err error) {
	cfg.Chain.ProducerPrivKey = hex.EncodeToString(identityset.PrivateKey(0).Bytes())
	if inMem {
		sf, err = factory.NewFactory(cfg, factory.InMemTrieOption())
		if err != nil {
			return
		}
	} else {
		sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
		if err != nil {
			return
		}
	}

	var indexerDB, blockdaoDB db.KVStore
	if inMem {
		indexerDB = db.NewMemKVStore()
		blockdaoDB = db.NewMemKVStore()
	} else {
		indexerDB = db.NewBoltDB(cfg.DB)
		blockdaoDB = db.NewBoltDB(cfg.DB)
	}
	// create indexer
	indexer, err = blockindex.NewIndexer(indexerDB, cfg.Genesis.Hash())
	if err != nil {
		return
	}
	// create BlockDAO
	dao = blockdao.NewBlockDAO(blockdaoDB, indexer, cfg.Chain.CompressBlock, cfg.DB)
	if dao == nil {
		return
	}
	// create chain
	registry = &protocol.Registry{}
	bc = blockchain.NewBlockchain(
		cfg,
		dao,
		blockchain.PrecreatedStateFactoryOption(sf),
		blockchain.RegistryOption(registry),
	)
	if bc == nil {
		return
	}
	defer func() {
		delete(cfg.Plugins, config.GatewayPlugin)
	}()
	var reward, acc, evm protocol.Protocol
	for _, protocol := range protocols {
		switch protocol {
		case rolldpos.ProtocolID:
			rolldposProtocol := rolldpos.NewProtocol(
				genesis.Default.NumCandidateDelegates,
				genesis.Default.NumDelegates,
				genesis.Default.NumSubEpochs,
			)
			reward = rewarding.NewProtocol(bc, rolldposProtocol)
			if err = registry.Register(rolldpos.ProtocolID, rolldposProtocol); err != nil {
				return
			}
		case account.ProtocolID:
			acc = account.NewProtocol(config.NewHeightUpgrade(cfg))
			if err = registry.Register(account.ProtocolID, acc); err != nil {
				return
			}
		case execution.ProtocolID:
			evm = execution.NewProtocol(bc, config.NewHeightUpgrade(cfg))
			if err = registry.Register(execution.ProtocolID, evm); err != nil {
				return
			}
		case rewarding.ProtocolID:
			if err = registry.Register(rewarding.ProtocolID, reward); err != nil {
				return
			}
		case poll.ProtocolID:
			p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
			if err = registry.Register(poll.ProtocolID, p); err != nil {
				return
			}
		}
	}

	sf.AddActionHandlers(acc, evm, reward)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	bc.Validator().AddActionValidators(acc, evm, reward)

	return
}
