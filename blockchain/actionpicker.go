// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool/actioniterator"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/state/factory"
)

// PickAction returns picked action list
func PickAction(gasLimit uint64, actionIterator actioniterator.ActionIterator) ([]action.SealedEnvelope, error) {
	pickedActions := make([]action.SealedEnvelope, 0)

	for {
		nextAction, ok := actionIterator.Next()
		if !ok {
			break
		}

		// use gaslimit for now, will change to real gas later
		gas := nextAction.GasLimit()
		if gasLimit < gas {
			break
		}
		gasLimit -= gas
		pickedActions = append(pickedActions, nextAction)
	}

	return pickedActions, nil
}

func CreateBlockchain(inMem bool, cfg config.Config, protocols []string) (bc Blockchain, dao blockdao.BlockDAO, indexer blockindex.Indexer, registry *protocol.Registry, sf factory.Factory, err error) {

	//// create indexer
	//cfg.DB.DbPath = cfg.Chain.IndexDBPath
	//indexer, err := blockindex.NewIndexer(db.NewBoltDB(cfg.DB), cfg.Genesis.Hash())
	//require.NoError(err)
	//// create BlockDAO
	//cfg.DB.DbPath = cfg.Chain.ChainDBPath
	//dao := blockdao.NewBlockDAO(db.NewBoltDB(cfg.DB), indexer, cfg.Chain.CompressBlock, cfg.DB)
	//require.NotNil(dao)
	//bc := NewBlockchain(
	//	cfg,
	//	dao,
	//	PrecreatedStateFactoryOption(sf),
	//	RegistryOption(&registry),
	//)
	//bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	//exec := execution.NewProtocol(bc, hc)
	//require.NoError(registry.Register(execution.ProtocolID, exec))
	//bc.Validator().AddActionValidators(acc, exec)
	//sf.AddActionHandlers(exec)
	//require.NoError(bc.Start(ctx))
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
		cfg.DB.DbPath = cfg.Chain.IndexDBPath
		indexerDB = db.NewBoltDB(cfg.DB)
		cfg.DB.DbPath = cfg.Chain.ChainDBPath
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

	var reward, acc, evm protocol.Protocol
	var rolldposProtocol *rolldpos.Protocol
	for _, protocol := range protocols {
		switch protocol {
		case rolldpos.ProtocolID:
			rolldposProtocol = rolldpos.NewProtocol(
				cfg.Genesis.NumCandidateDelegates,
				cfg.Genesis.NumDelegates,
				cfg.Genesis.NumSubEpochs,
			)
			if err = registry.Register(rolldpos.ProtocolID, rolldposProtocol); err != nil {
				return
			}

		case account.ProtocolID:
			acc = account.NewProtocol(config.NewHeightUpgrade(cfg))
			if err = registry.Register(account.ProtocolID, acc); err != nil {
				return
			}
			sf.AddActionHandlers(acc)
			bc.Validator().AddActionValidators(acc)
		case execution.ProtocolID:
			evm = execution.NewProtocol(bc, config.NewHeightUpgrade(cfg))
			if err = registry.Register(execution.ProtocolID, evm); err != nil {
				return
			}
			sf.AddActionHandlers(evm)
			bc.Validator().AddActionValidators(evm)
		case rewarding.ProtocolID:
			reward = rewarding.NewProtocol(bc, rolldposProtocol)
			if err = registry.Register(rewarding.ProtocolID, reward); err != nil {
				return
			}
			sf.AddActionHandlers(reward)
			bc.Validator().AddActionValidators(reward)
		case poll.ProtocolID:
			p := poll.NewLifeLongDelegatesProtocol(cfg.Genesis.Delegates)
			if err = registry.Register(poll.ProtocolID, p); err != nil {
				return
			}
		}
	}
	bc = NewBlockchain(
		cfg,
		dao,
		PrecreatedStateFactoryOption(sf),
		RegistryOption(registry),
	)
	if bc == nil {
		return
	}
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	if acc != nil {
		bc.Validator().AddActionValidators(acc)
	}
	if evm != nil {
		bc.Validator().AddActionValidators(evm)
	}
	if reward != nil {
		bc.Validator().AddActionValidators(reward)
	}
	return
}
