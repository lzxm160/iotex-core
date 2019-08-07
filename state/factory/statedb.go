// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"math/big"
	"strconv"
	"sync"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-core/state"
)

// stateDB implements StateFactory interface, tracks changes to account/contract and batch-commits to DB
type stateDB struct {
	mutex              sync.RWMutex
	currentChainHeight uint64
	dao                db.KVStore               // the underlying DB for account/contract storage
	actionHandlers     []protocol.ActionHandler // the handlers to handle actions
	timerFactory       *prometheustimer.TimerFactory
}

// StateDBOption sets stateDB construction parameter
type StateDBOption func(*stateDB, config.Config) error

// PrecreatedStateDBOption uses pre-created state db
func PrecreatedStateDBOption(kv db.KVStore) StateDBOption {
	return func(sdb *stateDB, cfg config.Config) error {
		if kv == nil {
			return errors.New("Invalid state db")
		}
		sdb.dao = kv
		return nil
	}
}

// DefaultStateDBOption creates default state db from config
func DefaultStateDBOption() StateDBOption {
	return func(sdb *stateDB, cfg config.Config) error {
		dbPath := cfg.Chain.TrieDBPath
		if len(dbPath) == 0 {
			return errors.New("Invalid empty trie db path")
		}
		cfg.DB.DbPath = dbPath // TODO: remove this after moving TrieDBPath from cfg.Chain to cfg.DB
		sdb.dao = db.NewBoltDB(cfg.DB)
		return nil
	}
}

// InMemStateDBOption creates in memory state db
func InMemStateDBOption() StateDBOption {
	return func(sdb *stateDB, cfg config.Config) error {
		sdb.dao = db.NewMemKVStore()
		return nil
	}
}

// NewStateDB creates a new state db
func NewStateDB(cfg config.Config, opts ...StateDBOption) (Factory, error) {
	sdb := stateDB{
		currentChainHeight: 0,
	}

	for _, opt := range opts {
		if err := opt(&sdb, cfg); err != nil {
			log.S().Errorf("Failed to execute state factory creation option %p: %v", opt, err)
			return nil, err
		}
	}
	timerFactory, err := prometheustimer.New(
		"iotex_statefactory_perf",
		"Performance of state factory module",
		[]string{"topic", "chainID"},
		[]string{"default", strconv.FormatUint(uint64(cfg.Chain.ID), 10)},
	)
	if err != nil {
		log.L().Error("Failed to generate prometheus timer factory.", zap.Error(err))
	}
	sdb.timerFactory = timerFactory
	return &sdb, nil
}

func (sdb *stateDB) Start(ctx context.Context) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	return sdb.dao.Start(ctx)
}

func (sdb *stateDB) Stop(ctx context.Context) error {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	return sdb.dao.Stop(ctx)
}

// AddActionHandlers adds action handlers to the state factory
func (sdb *stateDB) AddActionHandlers(actionHandlers ...protocol.ActionHandler) {
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	sdb.actionHandlers = append(sdb.actionHandlers, actionHandlers...)
}

//======================================
// account functions
//======================================
// Balance returns balance
func (sdb *stateDB) Balance(addr string) (*big.Int, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	account, err := sdb.accountState(addr)
	if err != nil {
		return nil, err
	}
	return account.Balance, nil
}

// Nonce returns the Nonce if the account exists
func (sdb *stateDB) Nonce(addr string) (uint64, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	account, err := sdb.accountState(addr)
	if err != nil {
		return 0, err
	}
	return account.Nonce, nil
}

// AccountState returns the confirmed account state on the chain
func (sdb *stateDB) AccountState(addr string) (*state.Account, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	return sdb.accountState(addr)
}

// RootHash returns the hash of the root node of the state trie
func (sdb *stateDB) RootHash() hash.Hash256 { return hash.ZeroHash256 }

// RootHashByHeight returns the hash of the root node of the state trie at a given height
func (sdb *stateDB) RootHashByHeight(blockHeight uint64) (hash.Hash256, error) {
	return hash.ZeroHash256, nil
}

// Height returns factory's height
func (sdb *stateDB) Height() (uint64, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	height, err := sdb.dao.Get(AccountKVNameSpace, []byte(CurrentHeightKey))
	if err != nil {
		return 0, errors.Wrap(err, "failed to get factory's height from underlying DB")
	}
	return byteutil.BytesToUint64(height), nil
}

func (sdb *stateDB) NewWorkingSet() (WorkingSet, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	return newStateTX(sdb.currentChainHeight, sdb.dao, sdb.actionHandlers), nil
}

// Commit persists all changes in RunActions() into the DB
func (sdb *stateDB) Commit(ws WorkingSet) error {
	if ws == nil {
		return errors.New("working set doesn't exist")
	}
	sdb.mutex.Lock()
	defer sdb.mutex.Unlock()
	timer := sdb.timerFactory.NewTimer("Commit")
	defer timer.End()
	if sdb.currentChainHeight != ws.Version() {
		// another working set with correct version already committed, do nothing
		return fmt.Errorf(
			"current state height %d doesn't match working set version %d",
			sdb.currentChainHeight,
			ws.Version(),
		)
	}
	if err := ws.Commit(); err != nil {
		return errors.Wrap(err, "failed to commit working set")
	}
	// Update chain height
	sdb.currentChainHeight = ws.Height()
	return nil
}

//======================================
// Candidate functions
//======================================
// CandidatesByHeight returns array of Candidates in candidate pool of a given height
func (sdb *stateDB) CandidatesByHeight(height uint64) ([]*state.Candidate, error) {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()
	var candidates state.CandidateList
	// Load Candidates on the given height from underlying db
	candidatesKey := candidatesutil.ConstructKey(height)
	err := sdb.state(candidatesKey, &candidates)
	log.L().Debug(
		"CandidatesByHeight",
		zap.Uint64("height", height),
		zap.Any("candidates", candidates),
		zap.Error(err),
	)
	if errors.Cause(err) == nil {
		if len(candidates) > 0 {
			return candidates, nil
		}
		err = state.ErrStateNotExist
	}
	return nil, errors.Wrapf(
		err,
		"failed to get state of candidateList for height %d",
		height,
	)
}

// State returns a confirmed state in the state factory
func (sdb *stateDB) State(addr hash.Hash160, state interface{}) error {
	sdb.mutex.RLock()
	defer sdb.mutex.RUnlock()

	return sdb.state(addr, state)
}

//======================================
// private trie constructor functions
//======================================

func (sdb *stateDB) state(addr hash.Hash160, s interface{}) error {
	data, err := sdb.dao.Get(AccountKVNameSpace, addr[:])
	if err != nil {
		if errors.Cause(err) == db.ErrNotExist {
			return errors.Wrapf(state.ErrStateNotExist, "state of %x doesn't exist", addr)
		}
		return errors.Wrapf(err, "error when getting the state of %x", addr)
	}
	if err := state.Deserialize(s, data); err != nil {
		return errors.Wrapf(err, "error when deserializing state data into %T", s)
	}
	return nil
}

func (sdb *stateDB) stateHeight(addr hash.Hash160, height uint64, s interface{}) error {
	//heightBytes := make([]byte, 8)
	//binary.BigEndian.PutUint64(heightBytes, height)
	//heightKey := append(addr[:], heightBytes...)

	maxVersion := uint64(0)
	indexKey := append(AccountMaxVersionPrefix, addr[:]...)
	value, err := sdb.dao.Get(AccountKVNameSpace, indexKey)
	if err == nil {
		maxVersion = binary.BigEndian.Uint64(value)
	}
	if maxVersion == 0 || height > maxVersion {
		return errors.New("cannot find state")
	}
	log.L().Info("////////////////", zap.Uint64("maxVersion", maxVersion))
	db := sdb.dao.DB()
	boltdb, ok := db.(*bolt.DB)
	if !ok {
		return errors.New("convert error")
	}
	boltdb.View(func(tx *bolt.Tx) error {
		c := tx.Bucket([]byte(AccountKVNameSpace)).Cursor()
		bytess := make([]byte, 8)
		binary.BigEndian.PutUint64(bytess, maxVersion)
		stateKey := append(addr[:], bytess...)
		for k, v := c.Seek(stateKey); k != nil && bytes.Compare(k, stateKey) <= 0; k, v = c.Prev() {
			kHeight := binary.BigEndian.Uint64(k[20:])
			log.L().Info("////////////////", zap.Uint64("k", kHeight), zap.Uint64("height", height))
			if kHeight == 0 {
				return errors.New("cannot find state")
			}
			if kHeight <= height {
				log.L().Info("////////////////", zap.Uint64("k", kHeight), zap.Uint64("height", height))
				if err := state.Deserialize(s, v); err != nil {
					return errors.Wrapf(err, "error when deserializing state data into %T", s)
				}
			}
		}
		return nil
	})

	return errors.New("cannot find state")
}

func (sdb *stateDB) accountState(encodedAddrs string) (account *state.Account, err error) {
	// TODO: state db shouldn't serve this function
	log.L().Info("////////////////", zap.String("address", encodedAddrs))

	height := uint64(0)
	if len(encodedAddrs) > 41 {
		height, err = strconv.ParseUint(encodedAddrs[41:], 10, 64)
		if err != nil {
			return
		}
		encodedAddrs = encodedAddrs[:41]
	}
	log.L().Info("////////////////", zap.Uint64("height", height), zap.String("address", encodedAddrs))
	addr, err := address.FromString(encodedAddrs)
	if err != nil {
		return nil, err
	}

	pkHash := hash.BytesToHash160(addr.Bytes())
	acc := state.EmptyAccount()
	account = &acc
	if height != 0 {
		err = sdb.stateHeight(pkHash, height, account)
		log.L().Info("////////////////stateHeight ", zap.Error(err))
	} else {
		fmt.Println("//////////////////320here")
		err = sdb.state(pkHash, account)
		fmt.Println("errrrrrrrrrrrrrrrrrr", err)
	}
	fmt.Println("//////////////////here")
	if err != nil {
		if errors.Cause(err) == state.ErrStateNotExist {
			acc = state.EmptyAccount()
			account = &acc
			return
		}
		err = errors.Wrapf(err, "error when loading state of %x", pkHash)
		return
	}
	return
}
