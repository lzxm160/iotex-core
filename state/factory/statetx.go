// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"context"
	"encoding/binary"
	"encoding/hex"

	"github.com/iotexproject/iotex-core/config"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// stateTX implements stateTX interface, tracks pending changes to account/contract in local cache
type stateTX struct {
	ver            uint64
	blkHeight      uint64
	cb             db.CachedBatch // cached batch for pending writes
	dao            db.KVStore     // the underlying DB for account/contract storage
	actionHandlers []protocol.ActionHandler
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

func (stx *stateTX) getMaxIndex(pkHash hash.Hash160) (uint64, error) {
	indexKey := append(AccountMaxHistoryIndexPrefix, pkHash[:]...)
	value, err := stx.dao.Get(AccountKVNameSpace, indexKey)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint64(value), nil
}
func (stx *stateTX) deleteIndex(pkHash hash.Hash160, maxIndex uint64) {
	indexHeightKey := append(AccountIndexHeightPrefix, pkHash[:]...)
	deleteHeight := stx.ver + 1 - stx.cfg.HistoryStateHeight
	for i := maxIndex; i > 0; i-- {
		indexBytes := make([]byte, 8)
		binary.BigEndian.PutUint64(indexBytes, i)
		HeightKey := append(indexHeightKey, indexBytes...)
		heightBytes, err := stx.dao.Get(AccountKVNameSpace, HeightKey)
		if err != nil {
			return
		}
		err = stx.dao.Delete(AccountKVNameSpace, HeightKey)
		if err != nil {
			return
		}
		hei := binary.BigEndian.Uint64(heightBytes)
		log.L().Info("////////////////deleteIndex", zap.Uint64("k", hei))
		if hei < deleteHeight {
			log.L().Info("////////////////", zap.Uint64("k", hei), zap.Uint64("deleteHeight", deleteHeight))
			stateKey := append(pkHash[:], heightBytes...)
			err := stx.dao.Delete(AccountKVNameSpace, stateKey)
			if err != nil {
				return
			}
		}
	}
}
func (stx *stateTX) putIndex(pkHash hash.Hash160, ss []byte) error {
	//stx.ver is last height,should be this block to pack action
	//binary.BigEndian.PutUint64(currentVersion, stx.ver+1)

	maxIndex, _ := stx.getMaxIndex(pkHash)
	//if (maxVersion != 0) && (maxVersion != 1) && (maxVersion > version) {
	//	return nil
	//}
	// add delete history later
	//stx.deleteIndex(pkHash, maxIndex)

	log.L().Info(
		"putIndex",
		zap.Uint64("stx.ver+1", stx.ver+1),
		zap.Uint64("index", maxIndex))
	currentIndex := make([]byte, 8)
	binary.BigEndian.PutUint64(currentIndex, maxIndex+1)
	indexKey := append(AccountMaxHistoryIndexPrefix, pkHash[:]...)
	err := stx.dao.Put(AccountKVNameSpace, indexKey, currentIndex)
	if err != nil {
		return err
	}
	version := stx.ver + 1
	currentHeight := make([]byte, 8)
	binary.BigEndian.PutUint64(currentHeight, version)
	// record index->height map
	indexHeightKey := append(AccountIndexHeightPrefix, pkHash[:]...)
	maxIndexBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(maxIndexBytes, maxIndex)
	indexHeightKey = append(indexHeightKey, maxIndexBytes...)
	err = stx.dao.Put(AccountKVNameSpace, indexHeightKey, currentHeight)
	if err != nil {
		return err
	}

	stateKey := append(pkHash[:], currentHeight...)
	log.L().Info(
		"put stateKey",zap.String("key:",hex.EncodeToString(stateKey)),zap.String("value:",hex.EncodeToString(ss)))
	return stx.dao.Put(AccountKVNameSpace, stateKey, ss)
}

// DelState deletes a state from DB
func (stx *stateTX) DelState(pkHash hash.Hash160) error {
	stx.cb.Delete(AccountKVNameSpace, pkHash[:], "error when deleting k = %x", pkHash)
	return nil
}
