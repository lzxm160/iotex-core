// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-core/state/factory"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
)

type (

	// StateDBAdapterRead represents the state db adapter for evm to access iotx blockchain
	StateDBAdapterRead struct {
		cm               protocol.ChainManager
		sm               protocol.StateManager
		sf               factory.Factory
		logs             []*action.Log
		err              error
		blockHeight      uint64
		executionHash    hash.Hash256
		refund           uint64
		cachedContract   contractMap
		contractSnapshot map[int]contractMap   // snapshots of contracts
		suicided         deleteAccount         // account/contract calling Suicide
		suicideSnapshot  map[int]deleteAccount // snapshots of suicide accounts
		preimages        preimageMap
		preimageSnapshot map[int]preimageMap
		dao              db.KVStore
		cb               db.CachedBatch
		hu               config.HeightUpgrade
	}
)

// NewStateDBAdapter creates a new state db with iotex blockchain
func NewStateDBAdapterRead(
	cm protocol.ChainManager,
	sm protocol.StateManager,
	sf factory.Factory,
	hu config.HeightUpgrade,
	blockHeight uint64,
	executionHash hash.Hash256,
) *StateDBAdapterRead {
	return &StateDBAdapterRead{
		cm:               cm,
		sm:               sm,
		sf:               sf,
		logs:             []*action.Log{},
		err:              nil,
		blockHeight:      blockHeight,
		executionHash:    executionHash,
		cachedContract:   make(contractMap),
		contractSnapshot: make(map[int]contractMap),
		suicided:         make(deleteAccount),
		suicideSnapshot:  make(map[int]deleteAccount),
		preimages:        make(preimageMap),
		preimageSnapshot: make(map[int]preimageMap),
		dao:              sm.GetDB(),
		cb:               sm.GetCachedBatch(),
		hu:               hu,
	}
}

func (stateDB *StateDBAdapterRead) logError(err error) {
	if stateDB.err == nil {
		stateDB.err = err
	}
}

// Error returns the first stored error during evm contract execution
func (stateDB *StateDBAdapterRead) Error() error {
	return stateDB.err
}

// CreateAccount creates an account in iotx blockchain
func (stateDB *StateDBAdapterRead) CreateAccount(evmAddr common.Address) {
	//addr, err := address.FromBytes(evmAddr.Bytes())
	//if err != nil {
	//	log.L().Error("Failed to convert evm address.", zap.Error(err))
	//	return
	//}
	//_, err = accountutil.LoadOrCreateAccount(stateDB.sm, addr.String(), big.NewInt(0))
	////if err != nil {
	////	log.L().Error("Failed to create account.", zap.Error(err))
	////	stateDB.logError(err)
	////	return
	////}
	////log.L().Debug("Called CreateAccount.", log.Hex("addrHash", evmAddr[:]))
	//hei := fmt.Sprintf("%d", stateDB.blockHeight)
	//stateDB.sm, _ = stateDB.sf.AccountState(addr.String() + hei)
}

// SubBalance subtracts balance from account
func (stateDB *StateDBAdapterRead) SubBalance(evmAddr common.Address, amount *big.Int) {

}

// AddBalance adds balance to account
func (stateDB *StateDBAdapterRead) AddBalance(evmAddr common.Address, amount *big.Int) {

}

// GetBalance gets the balance of account
func (stateDB *StateDBAdapterRead) GetBalance(evmAddr common.Address) *big.Int {
	return big.NewInt(9999999999999999)
}

// GetNonce gets the nonce of account
func (stateDB *StateDBAdapterRead) GetNonce(evmAddr common.Address) uint64 {
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return 0
	}
	state, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Error("Failed to get nonce.", zap.Error(err))
		// stateDB.logError(err)
		return 0
	}
	log.L().Debug("Called GetNonce.",
		zap.String("address", addr.String()),
		zap.Uint64("nonce", state.Nonce))
	return state.Nonce
}

// SetNonce sets the nonce of account
func (stateDB *StateDBAdapterRead) SetNonce(evmAddr common.Address, nonce uint64) {
}

// SubRefund subtracts refund
func (stateDB *StateDBAdapterRead) SubRefund(gas uint64) {
}

// AddRefund adds refund
func (stateDB *StateDBAdapterRead) AddRefund(gas uint64) {
}

// GetRefund gets refund
func (stateDB *StateDBAdapterRead) GetRefund() uint64 {
	return 0
}

// Suicide kills the contract
func (stateDB *StateDBAdapterRead) Suicide(evmAddr common.Address) bool {
	return true
}

// HasSuicided returns whether the contract has been killed
func (stateDB *StateDBAdapterRead) HasSuicided(evmAddr common.Address) bool {
	return false
}

// Exist checks the existence of an address
func (stateDB *StateDBAdapterRead) Exist(evmAddr common.Address) bool {
	return true
}

// Empty returns true if the the contract is empty
func (stateDB *StateDBAdapterRead) Empty(evmAddr common.Address) bool {
	return false
}

// RevertToSnapshot reverts the state factory to the state at a given snapshot
func (stateDB *StateDBAdapterRead) RevertToSnapshot(snapshot int) {
}

// Snapshot returns the snapshot id
func (stateDB *StateDBAdapterRead) Snapshot() int {
	return 0
}

// AddLog adds log
func (stateDB *StateDBAdapterRead) AddLog(evmLog *types.Log) {
}

// Logs returns the logs
func (stateDB *StateDBAdapterRead) Logs() []*action.Log {
	return stateDB.logs
}

// AddPreimage adds the preimage of a hash
func (stateDB *StateDBAdapterRead) AddPreimage(hash common.Hash, preimage []byte) {
}

// ForEachStorage loops each storage
func (stateDB *StateDBAdapterRead) ForEachStorage(addr common.Address, cb func(common.Hash, common.Hash) bool) error {
	addrs, err := address.FromBytes(addr[:])
	if err != nil {
		log.L().Error("ForEachStorage.", zap.Error(err), zap.String("addr", addr.String()))
		return err
	}
	ctt, err := stateDB.getContract(addrs.String())
	if err != nil {
		// stateDB.err = err
		return err
	}
	iter, err := ctt.Iterator()
	if err != nil {
		// stateDB.err = err
		return err
	}

	for {
		key, value, err := iter.Next()
		if err == trie.ErrEndOfIterator {
			// hit the end of the iterator, exit now
			return nil
		}
		if err != nil {
			return err
		}
		ckey := common.Hash{}
		copy(ckey[:], key[:])
		cvalue := common.Hash{}
		copy(cvalue[:], value[:])
		if !cb(ckey, cvalue) {
			return nil
		}
	}
	return nil
}

// AccountState returns an account state
func (stateDB *StateDBAdapterRead) AccountState(encodedAddr string) (*state.Account, error) {
	//addr, err := address.FromString(encodedAddr)
	//if err != nil {
	//	return nil, errors.Wrap(err, "failed to get public key hash from encoded address")
	//}
	//addrHash := hash.BytesToHash160(addr.Bytes())
	hei := fmt.Sprintf("%d", stateDB.blockHeight)
	return stateDB.sf.AccountState(encodedAddr + hei)
	//return accountutil.LoadAccount(stateDB.sm, addrHash)
}

//======================================
// Contract functions
//======================================

// GetCodeHash returns contract's code hash
func (stateDB *StateDBAdapterRead) GetCodeHash(evmAddr common.Address) common.Hash {
	codeHash := common.Hash{}
	addr, err := address.FromBytes(evmAddr[:])
	if err != nil {
		return codeHash
	}
	account, err := stateDB.AccountState(addr.String())
	copy(codeHash[:], account.CodeHash)
	return codeHash
}

// GetCode returns contract's code
func (stateDB *StateDBAdapterRead) GetCode(evmAddr common.Address) []byte {
	addr := hash.BytesToHash160(evmAddr[:])
	if contract, ok := stateDB.cachedContract[addr]; ok {
		code, err := contract.GetCode()
		if err != nil {
			log.L().Error("Failed to get code hash.", zap.Error(err))
			return nil
		}
		return code
	}
	account, err := accountutil.LoadAccount(stateDB.sm, addr)
	if err != nil {
		log.L().Error("Failed to load account state for address.", log.Hex("addrHash", addr[:]))
		return nil
	}
	code, err := stateDB.dao.Get(CodeKVNameSpace, account.CodeHash[:])
	if err != nil {
		// TODO: Suppress the as it's too much now
		//log.L().Error("Failed to get code from trie.", zap.Error(err))
		return nil
	}
	return code
}

// GetCodeSize gets the code size saved in hash
func (stateDB *StateDBAdapterRead) GetCodeSize(evmAddr common.Address) int {
	code := stateDB.GetCode(evmAddr)
	if code == nil {
		return 0
	}
	log.L().Debug("Called GetCodeSize.", log.Hex("addrHash", evmAddr[:]))
	return len(code)
}

// SetCode sets contract's code
func (stateDB *StateDBAdapterRead) SetCode(evmAddr common.Address, code []byte) {
}

// GetCommittedState gets committed state
func (stateDB *StateDBAdapterRead) GetCommittedState(evmAddr common.Address, k common.Hash) common.Hash {
	codeHash := common.Hash{}
	return codeHash
	//addr, err := address.FromBytes(evmAddr[:])
	//if err != nil {
	//	return codeHash
	//}
	////addr := hash.BytesToHash160(evmAddr[:])
	//contract, err := stateDB.getContract(addr.String())
	//if err != nil {
	//	log.L().Error("Failed to get contract.", zap.Error(err), zap.String("addrHash", addr.String()))
	//	stateDB.logError(err)
	//	return common.Hash{}
	//}
	//v, err := contract.GetCommittedState(hash.BytesToHash256(k[:]))
	//if err != nil {
	//	log.L().Error("Failed to get committed state.", zap.Error(err))
	//	stateDB.logError(err)
	//	return common.Hash{}
	//}
	//return common.BytesToHash(v)
}

// GetState gets state
func (stateDB *StateDBAdapterRead) GetState(evmAddr common.Address, k common.Hash) common.Hash {
	//addr := hash.BytesToHash160(evmAddr[:])
	//	//contract, err := stateDB.getContract(addr)
	//	//if err != nil {
	//	//	log.L().Error("Failed to get contract.", zap.Error(err), log.Hex("addrHash", addr[:]))
	//	//	stateDB.logError(err)
	//	//	return common.Hash{}
	//	//}
	codeHash := common.Hash{}
	addr, err := address.FromBytes(evmAddr[:])
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), zap.String("addrHash", addr.String()))
		return codeHash
	}
	contract, err := stateDB.getContract(addr.String())
	v, err := contract.GetState(hash.BytesToHash256(k[:]))
	if err != nil {
		log.L().Error("Failed to get state.", zap.Error(err))
		stateDB.logError(err)
		return common.Hash{}
	}
	return common.BytesToHash(v)
}

// SetState sets state
func (stateDB *StateDBAdapterRead) SetState(evmAddr common.Address, k, v common.Hash) {
}

// CommitContracts commits contract code to db and update pending contract account changes to trie
func (stateDB *StateDBAdapterRead) CommitContracts() error {
	return nil
}

// getContract returns the contract of addr
func (stateDB *StateDBAdapterRead) getContract(addr string) (Contract, error) {
	return stateDB.getNewContract(addr)
}

func (stateDB *StateDBAdapterRead) getNewContract(addr string) (Contract, error) {
	//account, err := accountutil.LoadAccount(stateDB.sm, addr)
	//if err != nil {
	//	return nil, errors.Wrapf(err, "failed to load account state for address %x", addr)
	//}
	account, err := stateDB.AccountState(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %s", addr)
	}
	a, err := address.FromString(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %s", addr)
	}
	contract, err := newContract(hash.Hash160b(a.Bytes()), account, stateDB.dao, stateDB.cb)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	return contract, nil
}
