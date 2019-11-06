// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package testutil

import (
	"math/big"

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

//// CreateBlockchain create blockchain without protocols
//func CreateBlockchain(inMem bool, cfg config.Config, protocols []string) (bc Blockchain, dao blockdao.BlockDAO, indexer blockindex.Indexer, registry *protocol.Registry, sf factory.Factory, err error) {
//	if inMem {
//		sf, err = factory.NewFactory(cfg, factory.InMemTrieOption())
//		if err != nil {
//			return
//		}
//	} else {
//		sf, err = factory.NewFactory(cfg, factory.DefaultTrieOption())
//		if err != nil {
//			return
//		}
//	}
//	var indexerDB, blockdaoDB db.KVStore
//	if inMem {
//		indexerDB = db.NewMemKVStore()
//		blockdaoDB = db.NewMemKVStore()
//	} else {
//		cfg.DB.DbPath = cfg.Chain.IndexDBPath
//		indexerDB = db.NewBoltDB(cfg.DB)
//		cfg.DB.DbPath = cfg.Chain.ChainDBPath
//		blockdaoDB = db.NewBoltDB(cfg.DB)
//	}
//	// create indexer
//	indexer, err = blockindex.NewIndexer(indexerDB, cfg.Genesis.Hash())
//	if err != nil {
//		return
//	}
//	// create BlockDAO
//	dao = blockdao.NewBlockDAO(blockdaoDB, indexer, cfg.Chain.CompressBlock, cfg.DB)
//	if dao == nil {
//		err = errors.New("failed to create blockdao")
//		return
//	}
//	// create chain
//	registry = &protocol.Registry{}
//	bc = NewBlockchain(
//		cfg,
//		dao,
//		PrecreatedStateFactoryOption(sf),
//		RegistryOption(registry),
//	)
//	if bc == nil {
//		err = errors.New("failed to create blockchain")
//		return
//	}
//
//	var reward, acc, evm protocol.Protocol
//	var rolldposProtocol *rolldpos.Protocol
//	var haveReward bool
//	for _, proto := range protocols {
//		switch proto {
//		case rolldpos.ProtocolID:
//			rolldposProtocol = rolldpos.NewProtocol(
//				cfg.Genesis.NumCandidateDelegates,
//				cfg.Genesis.NumDelegates,
//				cfg.Genesis.NumSubEpochs,
//			)
//			if err = registry.Register(rolldpos.ProtocolID, rolldposProtocol); err != nil {
//				return
//			}
//
//		case account.ProtocolID:
//			acc = account.NewProtocol(config.NewHeightUpgrade(cfg))
//			if err = registry.Register(account.ProtocolID, acc); err != nil {
//				return
//			}
//			sf.AddActionHandlers(acc)
//		case execution.ProtocolID:
//			evm = execution.NewProtocol(bc, config.NewHeightUpgrade(cfg))
//			if err = registry.Register(execution.ProtocolID, evm); err != nil {
//				return
//			}
//			sf.AddActionHandlers(evm)
//		case rewarding.ProtocolID:
//			haveReward = true
//		case poll.ProtocolID:
//			p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
//			if err = registry.Register(poll.ProtocolID, p); err != nil {
//				return
//			}
//		}
//	}
//
//	if haveReward && rolldposProtocol != nil {
//		reward = rewarding.NewProtocol(bc, rolldposProtocol)
//		if err = registry.Register(rewarding.ProtocolID, reward); err != nil {
//			return
//		}
//		sf.AddActionHandlers(reward)
//	}
//
//	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
//	if acc != nil {
//		bc.Validator().AddActionValidators(acc)
//	}
//	if evm != nil {
//		bc.Validator().AddActionValidators(evm)
//	}
//	if reward != nil {
//		bc.Validator().AddActionValidators(reward)
//	}
//	return
//}
