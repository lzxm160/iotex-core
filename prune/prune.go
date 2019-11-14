// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package prune

import (
	"context"
	"encoding/binary"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
)

var (
	// ErrInternalServer indicates the internal server error
	ErrInternalServer = errors.New("internal server error")
)

type (
	// Pruner is the interface for state Pruner
	Pruner interface {
		Start(context.Context) error
		Stop(context.Context) error
	}
)

// Prune provides service to do prune
type Prune struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    config.Config
	bc     blockchain.Blockchain
}

// NewPrune creates a new server
func NewPrune(cfg config.Config, bc blockchain.Blockchain) Pruner {
	return &Prune{
		cfg: cfg,
		bc:  bc,
	}
}

// Start starts the Prune server
func (p *Prune) Start(ctx context.Context) error {
	log.L().Info("Prune server is running.")
	p.ctx, p.cancel = context.WithCancel(ctx)
	go func() {
		if err := p.start(); err != nil {
			log.L().Fatal("Node failed to serve.", zap.Error(err))
		}
	}()
	return nil
}

// Stop stops the Prune server
func (p *Prune) Stop(ctx context.Context) error {
	p.cancel()
	return nil
}

func (p *Prune) start() error {
	d := time.Duration(1800) * time.Second
	t := time.NewTicker(d)
	defer t.Stop()
	for {
		select {
		case <-t.C:
			log.L().Info("Prune run ")
			err := p.prune()
			if err != nil {
				log.L().Error("Prune run error", zap.Error(err))
			}
		case <-p.ctx.Done():
			log.L().Info("Prune exit")
			return nil
		}
	}
}
func (p *Prune) prune() (err error) {
	log.L().Info("deleteHistory start")
	if p.bc == nil {
		return ErrInternalServer
	}
	currentHeight := p.bc.TipHeight()
	if currentHeight <= p.cfg.DB.HistoryStateRetention {
		return
	}
	deleteStartHeight := currentHeight - p.cfg.DB.HistoryStateRetention
	dao := p.bc.Factory()
	log.L().Info("////////////////deleteHistory", zap.Uint64("currentHeight", currentHeight), zap.Uint64("deleteStartHeight", deleteStartHeight))
	// find all keys that with version
	ws, err := dao.NewWorkingSet()
	if err != nil {
		return
	}
	allKeys, err := ws.GetDB().GetBucketByPrefix([]byte(factory.AccountKVNameSpace))
	if err != nil {
		log.L().Info("get prefix", zap.Error(err))
		return
	}
	//chaindbCache := db.NewCachedBatch()
	for _, key := range allKeys {
		ri, err := ws.GetDB().CreateRangeIndexNX(key, db.NotExist)
		if err != nil {
			continue
		}
		err = ri.Purge(deleteStartHeight)
		if err != nil {
			continue
		}
	}

	return p.deleteHistoryForTrie(deleteStartHeight, ws.GetDB())
}

// deleteHistoryForTrie delete account/state history asynchronous
func (p *Prune) deleteHistoryForTrie(hei uint64, dao db.KVStore) error {
	deleteStartHeight := hei
	var deleteEndHeight uint64
	if deleteStartHeight <= p.cfg.DB.HistoryStateRetention {
		deleteEndHeight = 1
	} else {
		deleteEndHeight = deleteStartHeight - p.cfg.DB.HistoryStateRetention
	}
	log.L().Info("deleteHeight", zap.Uint64("deleteStartHeight", deleteStartHeight), zap.Uint64("endHeight", deleteEndHeight), zap.Uint64("height", hei), zap.Uint64("historystateheight", p.cfg.DB.HistoryStateRetention))
	triedbCache := db.NewCachedBatch()
	for i := deleteStartHeight; i >= deleteEndHeight; i-- {
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, i)
		allKeys, err := dao.GetKeyByPrefix([]byte(factory.PruneKVNameSpace), heightBytes)
		if err != nil {
			continue
		}
		log.L().Info("deleteHeight", zap.Int("len(allKeys)", len(allKeys)), zap.Uint64("delete Height", i))
		for _, key := range allKeys {
			triedbCache.Delete(factory.PruneKVNameSpace, key, "failed to delete key %x", key)
			triedbCache.Delete(factory.ContractKVNameSpace, key[len(heightBytes):], "failed to delete key %x", key[len(heightBytes):])
		}
	}
	// delete trie node
	if err := dao.Commit(triedbCache); err != nil {
		return errors.Wrap(err, "failed to commit delete trie node")
	}
	return nil
}
