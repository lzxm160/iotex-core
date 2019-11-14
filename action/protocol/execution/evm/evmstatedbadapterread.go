// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package evm

import (
	"bytes"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

type (
	// StateDBAdapter represents the state db adapter for evm to access iotx blockchain
	StateDBAdapterRead struct {
		sf            factory.Factory
		getBlockHash  GetBlockHash
		sm            protocol.StateManager
		logs          []*action.Log
		err           error
		blockHeight   uint64
		executionHash hash.Hash256
		dao           db.KVStore
		cb            db.CachedBatch
		hu            config.HeightUpgrade
		saveHistory   bool
	}
)

// NewStateDBAdapter creates a new state db with iotex blockchain
func NewStateDBAdapterRead(
	sf factory.Factory,
	getBlockHash GetBlockHash,
	sm protocol.StateManager,
	hu config.HeightUpgrade,
	blockHeight uint64,
	executionHash hash.Hash256,
	opts ...StateDBOption,
) *StateDBAdapterRead {
	s := &StateDBAdapterRead{
		sf:            sf,
		getBlockHash:  getBlockHash,
		sm:            sm,
		logs:          []*action.Log{},
		err:           nil,
		blockHeight:   blockHeight,
		executionHash: executionHash,
		dao:           sm.GetDB(),
		cb:            sm.GetCachedBatch(),
		hu:            hu,
		saveHistory:   true,
	}
	return s
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
	log.L().Info("StateDBAdapterRead Called CreateAccount.")
}

// SubBalance subtracts balance from account
func (stateDB *StateDBAdapterRead) SubBalance(evmAddr common.Address, amount *big.Int) {
	log.L().Info("StateDBAdapterRead Called SubBalance.")
}

// AddBalance adds balance to account
func (stateDB *StateDBAdapterRead) AddBalance(evmAddr common.Address, amount *big.Int) {
	log.L().Info("StateDBAdapterRead Called AddBalance.")
}

// GetBalance gets the balance of account
func (stateDB *StateDBAdapterRead) GetBalance(evmAddr common.Address) *big.Int {
	log.L().Info("StateDBAdapterRead Called GetBalance.")
	return big.NewInt(9999999999999999)
}

// GetNonce gets the nonce of account
func (stateDB *StateDBAdapterRead) GetNonce(evmAddr common.Address) uint64 {
	log.L().Info("StateDBAdapterRead Called GetNonce.")
	addr, err := address.FromBytes(evmAddr.Bytes())
	if err != nil {
		log.L().Error("Failed to convert evm address.", zap.Error(err))
		return 0
	}
	state, err := stateDB.AccountState(addr.String())
	if err != nil {
		log.L().Error("Failed to get nonce.", zap.Error(err))
		return 0
	}
	return state.Nonce
}

// SetNonce sets the nonce of account
func (stateDB *StateDBAdapterRead) SetNonce(evmAddr common.Address, nonce uint64) {
	log.L().Info("StateDBAdapterRead Called SetNonce.")
}

// SubRefund subtracts refund
func (stateDB *StateDBAdapterRead) SubRefund(gas uint64) {
	log.L().Info("StateDBAdapterRead Called SubRefund.")
}

// AddRefund adds refund
func (stateDB *StateDBAdapterRead) AddRefund(gas uint64) {
	log.L().Info("StateDBAdapterRead Called AddRefund.")
}

// GetRefund gets refund
func (stateDB *StateDBAdapterRead) GetRefund() uint64 {
	log.L().Info("StateDBAdapterRead Called GetRefund.")
	return 0
}

// Suicide kills the contract
func (stateDB *StateDBAdapterRead) Suicide(evmAddr common.Address) bool {
	return false
}

// HasSuicided returns whether the contract has been killed
func (stateDB *StateDBAdapterRead) HasSuicided(evmAddr common.Address) bool {
	return false
}

// Exist checks the existence of an address
func (stateDB *StateDBAdapterRead) Exist(evmAddr common.Address) bool {
	log.L().Info("StateDBAdapterRead Called Exist.")
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
	log.L().Info("StateDBAdapterRead Called ForEachStorage.")
	return nil
}

// AccountState returns an account state
func (stateDB *StateDBAdapterRead) AccountState(encodedAddr string) (*state.Account, error) {
	log.L().Info("StateDBAdapterRead Called AccountState.", zap.Uint64("height", stateDB.blockHeight))
	hei := fmt.Sprintf("%d", stateDB.blockHeight)
	//return stateDB.sm..StateByAddr(encodedAddr + hei)
	//return accountutil.LoadAccount(stateDB.sm, addrHash)
	return stateDB.sf.AccountState(encodedAddr + hei)
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
	if err != nil {
		return codeHash
	}
	copy(codeHash[:], account.CodeHash)
	return codeHash
}

// GetCode returns contract's code
func (stateDB *StateDBAdapterRead) GetCode(evmAddr common.Address) []byte {
	h := stateDB.GetCodeHash(evmAddr)
	codeHash := common.Hash{}
	if bytes.Equal(h[:], codeHash[:]) {
		return nil
	}
	code, err := stateDB.dao.Get(factory.CodeKVNameSpace, h[:])
	if err != nil {
		log.L().Error("Called GetCode.", zap.Error(err))
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
}

// GetState gets state
func (stateDB *StateDBAdapterRead) GetState(evmAddr common.Address, k common.Hash) common.Hash {
	log.L().Info("StateDBAdapterRead Called GetState.")
	codeHash := common.Hash{}
	addr, err := address.FromBytes(evmAddr[:])
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), zap.String("addrHash", addr.String()))
		return codeHash
	}
	contract, err := stateDB.getContract(addr.String())
	if err != nil {
		log.L().Error("Failed to get contract.", zap.Error(err), zap.String("addr", addr.String()))
		stateDB.logError(err)
		return common.Hash{}
	}
	v, err := contract.GetState(hash.BytesToHash256(k[:]))
	if err != nil {
		log.L().Debug("Failed to get state.", zap.Error(err))
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
	log.L().Info("StateDBAdapterRead Called getNewContract.")
	account, err := stateDB.AccountState(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %s", addr)
	}
	a, err := address.FromString(addr)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to load account state for address %s", addr)
	}
	contract, err := newContract(hash.Hash160b(a.Bytes()), account, stateDB.dao, stateDB.cb, HistoryRetentionOption(stateDB.blockHeight))

	if err != nil {
		return nil, errors.Wrapf(err, "failed to create storage trie for new contract %x", addr)
	}
	return contract, nil
}

// clear clears local changes
func (stateDB *StateDBAdapterRead) clear() {
}
