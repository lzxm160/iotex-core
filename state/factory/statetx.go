// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"encoding/binary"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

const (
	// CheckHistoryDeleteInterval 30 block heights to check if history needs to delete
	CheckHistoryDeleteInterval = 100
)

// stateTX implements stateTX interface, tracks pending changes to account/contract in local cache
type stateTX struct {
	ver            uint64
	blkHeight      uint64
	cb             db.CachedBatch // cached batch for pending writes
	dao            db.KVStore     // the underlying DB for account/contract storage
	actionHandlers []protocol.ActionHandler
	deleting       chan struct{} // make sure there's only one goroutine deleting history state
	cfg            config.DB
}

// newStateTX creates a new state tx
func newStateTX(
	version uint64,
	kv db.KVStore,
	actionHandlers []protocol.ActionHandler,
	cfg config.DB,
) *stateTX {
	return &stateTX{
		ver:            version,
		cb:             db.NewCachedBatch(),
		dao:            kv,
		actionHandlers: actionHandlers,
		deleting:       make(chan struct{}, 1),
		cfg:            cfg,
	}
}

// RootHash returns the hash of the root node of the accountTrie
func (stx *stateTX) RootHash() hash.Hash256 { return hash.ZeroHash256 }

// Digest returns the delta state digest
func (stx *stateTX) Digest() hash.Hash256 { return stx.GetCachedBatch().Digest() }

// Version returns the Version of this working set
func (stx *stateTX) Version() uint64 { return stx.ver }

// Height returns the Height of the block being worked on
func (stx *stateTX) Height() uint64 { return stx.blkHeight }

// RunActions runs actions in the block and track pending changes in working set
func (stx *stateTX) RunActions(
	ctx context.Context,
	blockHeight uint64,
	elps []action.SealedEnvelope,
) ([]*action.Receipt, error) {
	// Handle actions
	receipts := make([]*action.Receipt, 0)
	var raCtx protocol.RunActionsCtx
	if len(elps) > 0 {
		raCtx = protocol.MustGetRunActionsCtx(ctx)
	}
	for _, elp := range elps {
		receipt, err := stx.RunAction(raCtx, elp)
		if err != nil {
			return nil, errors.Wrap(err, "error when run action")
		}
		if receipt != nil {
			receipts = append(receipts, receipt)
		}
	}
	stx.UpdateBlockLevelInfo(blockHeight)
	return receipts, nil
}

// RunAction runs action in the block and track pending changes in working set
func (stx *stateTX) RunAction(
	raCtx protocol.RunActionsCtx,
	elp action.SealedEnvelope,
) (*action.Receipt, error) {
	// Handle action
	// Add caller address into the run action context
	callerAddr, err := address.FromBytes(elp.SrcPubkey().Hash())
	if err != nil {
		return nil, err
	}
	raCtx.Caller = callerAddr
	raCtx.ActionHash = elp.Hash()
	raCtx.GasPrice = elp.GasPrice()
	intrinsicGas, err := elp.IntrinsicGas()
	if err != nil {
		return nil, err
	}
	raCtx.IntrinsicGas = intrinsicGas
	raCtx.Nonce = elp.Nonce()
	ctx := protocol.WithRunActionsCtx(context.Background(), raCtx)
	for _, actionHandler := range stx.actionHandlers {
		receipt, err := actionHandler.Handle(ctx, elp.Action(), stx)
		if err != nil {
			return nil, errors.Wrapf(
				err,
				"error when action %x (nonce: %d) from %s mutates states",
				elp.Hash(),
				elp.Nonce(),
				callerAddr.String(),
			)
		}
		if receipt != nil {
			return receipt, nil
		}
	}
	return nil, nil
}

// UpdateBlockLevelInfo runs action in the block and track pending changes in working set
func (stx *stateTX) UpdateBlockLevelInfo(blockHeight uint64) hash.Hash256 {
	if stx.cfg.EnableHistoryState && blockHeight%CheckHistoryDeleteInterval == 0 && blockHeight != 0 {
		stx.deleteHistory()
	}
	stx.blkHeight = blockHeight
	// Persist current chain Height
	h := byteutil.Uint64ToBytes(blockHeight)
	stx.cb.Put(AccountKVNameSpace, []byte(CurrentHeightKey), h, "failed to store accountTrie's current Height")
	return hash.ZeroHash256
}

func (stx *stateTX) Snapshot() int { return stx.cb.Snapshot() }

func (stx *stateTX) Revert(snapshot int) error { return stx.cb.Revert(snapshot) }

// Commit persists all changes in RunActions() into the DB
func (stx *stateTX) Commit() error {
	// Commit all changes in a batch
	dbBatchSizelMtc.WithLabelValues().Set(float64(stx.cb.Size()))
	if err := stx.dao.Commit(stx.cb); err != nil {
		return errors.Wrap(err, "failed to Commit all changes to underlying DB in a batch")
	}
	return nil
}

// GetDB returns the underlying DB for account/contract storage
func (stx *stateTX) GetDB() db.KVStore {
	return stx.dao
}

// GetCachedBatch returns the cached batch for pending writes
func (stx *stateTX) GetCachedBatch() db.CachedBatch {
	return stx.cb
}

// State pulls a state from DB
func (stx *stateTX) State(hash hash.Hash160, s interface{}) error {
	stateDBMtc.WithLabelValues("get").Inc()
	mstate, err := stx.cb.Get(AccountKVNameSpace, hash[:])
	if errors.Cause(err) == db.ErrNotExist {
		if mstate, err = stx.dao.Get(AccountKVNameSpace, hash[:]); errors.Cause(err) == db.ErrNotExist {
			return errors.Wrapf(state.ErrStateNotExist, "k = %x doesn't exist", hash)
		}
	}
	if errors.Cause(err) == db.ErrAlreadyDeleted {
		return errors.Wrapf(state.ErrStateNotExist, "k = %x doesn't exist", hash)
	}
	if err != nil {
		return errors.Wrapf(err, "failed to get account of %x", hash)
	}
	return state.Deserialize(s, mstate)
}

// PutState puts a state into DB
func (stx *stateTX) PutState(pkHash hash.Hash160, s interface{}) error {
	stateDBMtc.WithLabelValues("put").Inc()
	ss, err := state.Serialize(s)
	if err != nil {
		return errors.Wrapf(err, "failed to convert account %v to bytes", s)
	}
	stx.cb.Put(AccountKVNameSpace, pkHash[:], ss, "error when putting k = %x", pkHash)
	if !stx.cfg.EnableHistoryState {
		return nil
	}
	return stx.putIndex(pkHash, ss)
}

func (stx *stateTX) getMaxVersion(pkHash hash.Hash160) (index, height uint64, err error) {
	indexKey := append(AccountMaxVersionPrefix, pkHash[:]...)
	value, err := stx.dao.Get(AccountKVNameSpace, indexKey)
	if err != nil {
		return
	}
	index = binary.BigEndian.Uint64(value[:8])
	height = binary.BigEndian.Uint64(value[8:])
	return
}

func (stx *stateTX) putIndex(pkHash hash.Hash160, ss []byte) error {
	version := stx.ver + 1
	maxIndex, maxHeight, _ := stx.getMaxVersion(pkHash)
	if (maxHeight != 0) && (maxHeight != 1) && (maxHeight > version) {
		return nil
	}
	log.L().Info("////////////////putIndex", zap.Uint64("maxIndex", maxIndex), zap.Uint64("maxHeight", maxHeight))
	// index from 0
	currentIndex := make([]byte, 8)
	binary.BigEndian.PutUint64(currentIndex, maxIndex)
	currentHeight := make([]byte, 8)
	binary.BigEndian.PutUint64(currentHeight, version)
	indexKey := append(pkHash[:], AccountIndexPrefix...)
	indexKey = append(indexKey, currentIndex...)
	// put accounthash+AccountIndexPrefix+index->height
	err := stx.dao.Put(AccountKVNameSpace, indexKey, currentHeight)
	if err != nil {
		return err
	}

	// max num of index
	maxIndex++
	binary.BigEndian.PutUint64(currentIndex, maxIndex)
	versionValue := append(currentIndex, currentHeight...)
	maxIndexKey := append(AccountMaxVersionPrefix, pkHash[:]...)
	// put AccountMaxVersionPrefix+accounthash->index+height
	err = stx.dao.Put(AccountKVNameSpace, maxIndexKey, versionValue)
	if err != nil {
		return err
	}
	stateKey := append(pkHash[:], currentHeight...)
	return stx.dao.Put(AccountKVNameSpace, stateKey, ss)
}

// delete history asynchronous,this will find all account that with version
func (stx *stateTX) deleteHistory() error {
	log.L().Info("deleteHistory start")
	currentHeight := stx.ver + 1
	if currentHeight <= stx.cfg.HistoryStateHeight {
		return nil
	}
	deleteStartHeight := currentHeight - stx.cfg.HistoryStateHeight
	var deleteEndHeight uint64
	if deleteStartHeight < stx.cfg.HistoryStateHeight {
		deleteEndHeight = 1
	} else {
		deleteEndHeight = deleteStartHeight - stx.cfg.HistoryStateHeight
	}
	go func() {
		stx.deleting <- struct{}{}
		// find all keys that with version
		allKeys, err := stx.dao.GetPrefix(AccountKVNameSpace, AccountMaxVersionPrefix)
		if err != nil {
			return
		}
		chaindbCache := db.NewCachedBatch()
		for _, key := range allKeys {
			addrHash := key[len(AccountMaxVersionPrefix):]
			pkHash := hash.BytesToHash160(addrHash)
			maxIndex, maxHeight, err := stx.getMaxVersion(pkHash)
			if err != nil {
				continue
			}
			if maxHeight < deleteEndHeight {
				continue
			}
			for i := maxIndex; i >= 0; i-- {
				currentIndex := make([]byte, 8)
				binary.BigEndian.PutUint64(currentIndex, i)
				indexKey := append(pkHash[:], AccountIndexPrefix...)
				indexKey = append(indexKey, currentIndex...)
				// put accounthash+AccountIndexPrefix+index->height
				height, err := stx.dao.Get(AccountKVNameSpace, indexKey)
				if err != nil {
					break
				}
				indexHeight := binary.BigEndian.Uint64(height[:])
				if indexHeight <= deleteEndHeight && indexHeight < deleteStartHeight {
					log.L().Info("////////////////deleteHistory", zap.Uint64("indexHeight", indexHeight))
					chaindbCache.Delete(AccountKVNameSpace, indexKey, "")
					//height:=binary.BigEndian.Uint64(value[:8])
					accountHeight := append(addrHash, height...)
					chaindbCache.Delete(AccountKVNameSpace, accountHeight, "")
				}
			}
		}
		if err := stx.dao.Commit(chaindbCache); err != nil {
			log.L().Error("failed to commit delete account history", zap.Error(err))
			return
		}
		<-stx.deleting
	}()
	return nil
}

// DeleteHistoryForTrie delete history asynchronous for trie node
func (stx *stateTX) DeleteHistoryForTrie(hei uint64, namespace string, prefix []byte, chaindb db.KVStore) error {
	if hei < stx.cfg.HistoryStateHeight {
		return nil
	}
	deleteStartHeight := hei - stx.cfg.HistoryStateHeight
	var deleteEndHeight uint64
	if deleteStartHeight < stx.cfg.HistoryStateHeight {
		deleteEndHeight = 1
	} else {
		deleteEndHeight = deleteStartHeight - stx.cfg.HistoryStateHeight
	}
	log.L().Info("deleteHeight", zap.Uint64("deleteStartHeight", deleteStartHeight), zap.Uint64("endHeight", deleteEndHeight), zap.Uint64("height", hei), zap.Uint64("historystateheight", stx.cfg.HistoryStateHeight))
	chaindbCache := db.NewCachedBatch()
	triedbCache := db.NewCachedBatch()
	for i := deleteStartHeight; i >= deleteEndHeight; i-- {
		heightBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(heightBytes, i)
		keyPrefix := append(prefix, heightBytes...)
		allKeys, err := stx.dao.GetPrefix(namespace, keyPrefix)
		if err != nil {
			continue
		}
		for _, key := range allKeys {
			chaindbCache.Delete(namespace, key, "failed to delete key %x", key)
			triedbCache.Delete(db.ContractKVNameSpace, key[len(keyPrefix):], "failed to delete key %x", key[len(keyPrefix):])
		}
	}
	// delete trie node reference
	if err := chaindb.Commit(chaindbCache); err != nil {
		return errors.Wrap(err, "failed to commit delete trie node reference")
	}
	// delete trie node
	if err := stx.dao.Commit(triedbCache); err != nil {
		return errors.Wrap(err, "failed to commit delete trie node")
	}
	return nil
}

// DelState deletes a state from DB
func (stx *stateTX) DelState(pkHash hash.Hash160) error {
	stx.cb.Delete(AccountKVNameSpace, pkHash[:], "error when deleting k = %x", pkHash)
	return nil
}
