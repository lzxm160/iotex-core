// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/state"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestProtocol_HandleCreateStake(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm := newMockStateManager(ctrl)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)

	// create protocol
	p, err := NewProtocol(depositGas, sm, genesis.Default.Staking)
	require.NoError(err)

	// set up candidate
	candidate := testCandidates[0].d.Clone()
	require.NoError(setupCandidate(p, sm, candidate))
	candidateName := candidate.Name
	candidateAddr := candidate.Owner

	stakerAddr := identityset.Address(1)
	tests := []struct {
		// action fields
		initBalance int64
		candName    string
		amount      string
		duration    uint32
		autoStake   bool
		gasPrice    *big.Int
		gasLimit    uint64
		nonce       uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// expected result
		status iotextypes.ReceiptStatus
	}{
		{
			10,
			candidateName,
			"10000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		{
			100,
			"notExist",
			"10000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		{
			100,
			candidateName,
			"10000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		require.NoError(setupAccount(sm, stakerAddr, test.initBalance))
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       stakerAddr,
			GasPrice:     test.gasPrice,
			IntrinsicGas: test.gasLimit,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    test.blkHeight,
			BlockTimeStamp: test.blkTimestamp,
			GasLimit:       test.blkGasLimit,
		})
		act, err := action.NewCreateStake(test.nonce, test.candName, test.amount, test.duration, test.autoStake,
			nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		r, err := p.handleCreateStake(ctx, act, sm)
		require.Equal(uint64(test.status), r.Status)

		if test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, err := getCandBucketIndices(sm, candidateAddr)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, err = getVoterBucketIndices(sm, stakerAddr)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := getBucket(sm, indices[0])
			require.NoError(err)
			require.Equal(candidateAddr, bucket.Candidate)
			require.Equal(stakerAddr, bucket.Owner)
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, err := getCandidate(sm, candidateAddr)
			require.NoError(err)
			require.LessOrEqual(test.amount, candidate.Votes.String())
			candidate = p.inMemCandidates.GetByOwner(candidateAddr)
			require.NotNil(candidate)
			require.LessOrEqual(test.amount, candidate.Votes.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, hash.BytesToHash160(stakerAddr.Bytes()))
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), big.NewInt(0).Add(caller.Balance, actCost))
			require.Equal(test.nonce, caller.Nonce)
		}
	}
}

func TestProtocol_HandleCandidateRegister(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm, p, _, _ := initAll(t, ctrl)
	tests := []struct {
		initBalance     int64
		caller          address.Address
		nonce           uint64
		name            string
		operatorAddrStr string
		rewardAddrStr   string
		ownerAddrStr    string
		amountStr       string
		duration        uint32
		autoStake       bool
		payload         []byte
		gasLimit        uint64
		blkGasLimit     uint64
		gasPrice        *big.Int
		newProtocol     bool
		err             error
		status          iotextypes.ReceiptStatus
	}{
		// fetchCaller,ErrNotEnoughBalance
		{
			100,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"100",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// settleAction,ErrHitGasLimit
		{
			1000,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"100",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000),
			big.NewInt(1000),
			true,
			action.ErrHitGasLimit,
			iotextypes.ReceiptStatus_Success,
		},
		// Upsert,check collision
		{
			1000,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"100",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			false,
			ErrInvalidSelfStkIndex,
			iotextypes.ReceiptStatus_Success,
		},
		// owner address is nil
		{
			101,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			"",
			"1",
			uint32(10000),
			false,
			nil,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1),
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
		{
			101,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"1",
			uint32(10000),
			false,
			nil,
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1),
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		if test.newProtocol {
			sm, p, _, _ = initAll(t, ctrl)
		}
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		act, err := action.NewCandidateRegister(test.nonce, test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		IntrinsicGas, _ := act.IntrinsicGas()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: IntrinsicGas,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       test.blkGasLimit,
		})
		r, err := p.handleCandidateRegister(ctx, act, sm)
		if err != nil {
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		}

		if err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test candidate
			candidate, err := getCandidate(sm, act.OwnerAddress())
			if act.OwnerAddress() == nil {
				require.Nil(candidate)
				require.Equal(ErrNilParameters, errors.Cause(err))
				candidate, err = getCandidate(sm, test.caller)
				require.NoError(err)
				require.Equal(test.caller.String(), candidate.Owner.String())
			} else {
				require.NotNil(candidate)
				require.NoError(err)
				require.Equal(test.ownerAddrStr, candidate.Owner.String())
			}
			require.LessOrEqual("0", candidate.Votes.String())
			candidate = p.inMemCandidates.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.LessOrEqual("0", candidate.Votes.String())
			require.Equal(test.name, candidate.Name)
			require.Equal(test.operatorAddrStr, candidate.Operator.String())
			require.Equal(test.rewardAddrStr, candidate.Reward.String())
			require.Equal(test.amountStr, candidate.Votes.String())
			require.Equal(test.amountStr, candidate.SelfStake.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, hash.BytesToHash160(test.caller.Bytes()))
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, p.config.RegistrationConsts.Fee))
			require.Equal(test.nonce, caller.Nonce)
		}
	}
}

func TestProtocol_handleCandidateUpdate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tests := []struct {
		initBalance     int64
		caller          address.Address
		nonce           uint64
		name            string
		operatorAddrStr string
		rewardAddrStr   string
		ownerAddrStr    string
		amountStr       string
		duration        uint32
		autoStake       bool
		payload         []byte
		gasLimit        uint64
		blkGasLimit     uint64
		gasPrice        *big.Int
		newProtocol     bool
		// candidate update
		updateName     string
		updateOperator string
		updateReward   string
		err            error
		status         iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			110,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(27).String(),
			"9999999999989300000",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"update",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// only owner can update candidate
		{
			1000,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			identityset.Address(30).String(),
			"100",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"update",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			ErrInvalidOwner,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// ReceiptStatus_ErrCandidateNotExist
		{
			1000,
			identityset.Address(28),
			uint64(10),
			"test",
			identityset.Address(28).String(),
			identityset.Address(29).String(),
			"",
			"100",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"update",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			nil,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		{
			1000,
			identityset.Address(27),
			uint64(10),
			"test",
			identityset.Address(27).String(),
			identityset.Address(29).String(),
			identityset.Address(27).String(),
			"100",
			uint32(10000),
			false,
			[]byte("payload"),
			uint64(1000000),
			uint64(1000000),
			big.NewInt(1000),
			true,
			"update",
			identityset.Address(31).String(),
			identityset.Address(32).String(),
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, _, _ := initAll(t, ctrl)
		require.NoError(setupAccount(sm, identityset.Address(28), test.initBalance))
		require.NoError(setupAccount(sm, identityset.Address(27), test.initBalance))
		act, err := action.NewCandidateRegister(test.nonce, test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, _ := act.IntrinsicGas()
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       identityset.Address(27),
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       test.blkGasLimit,
		})
		_, err = p.handleCandidateRegister(ctx, act, sm)
		require.NoError(err)

		cu, err := action.NewCandidateUpdate(test.nonce, test.updateName, test.updateOperator, test.updateReward, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, _ = cu.IntrinsicGas()
		ctx = protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       test.blkGasLimit,
		})
		r, err := p.handleCandidateUpdate(ctx, cu, sm)
		if err != nil {
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		}

		if err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test candidate
			candidate, err := getCandidate(sm, act.OwnerAddress())
			if act.OwnerAddress() == nil {
				require.Nil(candidate)
				require.Equal(ErrNilParameters, errors.Cause(err))
				candidate, err = getCandidate(sm, test.caller)
				require.NoError(err)
				require.Equal(test.caller.String(), candidate.Owner.String())
			} else {
				require.NotNil(candidate)
				require.NoError(err)
				require.Equal(test.ownerAddrStr, candidate.Owner.String())
			}
			require.LessOrEqual("0", candidate.Votes.String())
			candidate = p.inMemCandidates.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.LessOrEqual("0", candidate.Votes.String())
			require.Equal(test.updateName, candidate.Name)
			require.Equal(test.updateOperator, candidate.Operator.String())
			require.Equal(test.updateReward, candidate.Reward.String())
			require.LessOrEqual(test.amountStr, candidate.Votes.String())
			require.Equal(test.amountStr, candidate.SelfStake.String())

			// test staker's account
			caller, err := accountutil.LoadAccount(sm, hash.BytesToHash160(test.caller.Bytes()))
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			cuCost, err := cu.Cost()
			require.NoError(err)
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, cuCost).Add(total, p.config.RegistrationConsts.Fee))
			require.Equal(test.nonce, caller.Nonce)
		}
	}
}

func TestProtocol_HandleUnstake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm, p, candidate, candidate2 := initAll(t, ctrl)
	ctx, _ := initCreateStake(t, sm, identityset.Address(2), 100, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, "10000000000000000000")

	callerAddr := identityset.Address(1)
	tests := []struct {
		// creat stake fields
		caller      address.Address
		amount      string
		initBalance int64
		selfstaking bool
		// action fields
		index    uint64
		gasPrice *big.Int
		gasLimit uint64
		nonce    uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// clear flag for inMemCandidates
		clear bool
		// need new p
		newProtocol bool
		err         error
		// expected result
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			callerAddr,
			"9990000000000000000",
			10,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// for bucket.Owner is not equal to actionCtx.Caller
		{
			identityset.Address(12),
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			false,
			ErrFetchBucket,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// updateBucket getbucket ErrStateNotExist
		{
			identityset.Address(33),
			"10000000000000000000",
			100,
			false,
			1,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			true,
			state.ErrStateNotExist,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		},
		// for inMemCandidates.GetByOwner,ErrInvalidOwner
		{
			callerAddr,
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			true,
			true,
			ErrInvalidOwner,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		{
			callerAddr,
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		if test.newProtocol {
			sm, p, candidate, _ = initAll(t, ctrl)
		} else {
			candidate = candidate2
		}
		ctx, _ = initCreateStake(t, sm, test.caller, test.initBalance, test.gasPrice, test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candidate, test.amount)
		fmt.Println(candidate.Name)
		act, err := action.NewUnstake(test.nonce, test.index,
			nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		if test.clear {
			p.inMemCandidates.Delete(test.caller)
		}
		r, err := p.handleUnstake(ctx, act, sm)
		if err != nil {
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		}

		if err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, err := getCandBucketIndices(sm, candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, err = getVoterBucketIndices(sm, candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := getBucket(sm, indices[0])
			require.NoError(err)
			require.Equal(candidate.Owner.String(), bucket.Candidate.String())
			require.Equal(test.caller.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, err = getCandidate(sm, candidate.Owner)
			require.NoError(err)
			require.Equal("2", candidate.Votes.String())
			candidate = p.inMemCandidates.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal("2", candidate.Votes.String())
		}

	}
}

func TestProtocol_HandleWithdrawStake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	tests := []struct {
		// create stake fields
		caller      address.Address
		amount      string
		initBalance int64
		selfstaking bool
		// action fields
		index    uint64
		gasPrice *big.Int
		gasLimit uint64
		nonce    uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		ctxTimestamp time.Time
		blkGasLimit  uint64
		// if unstake
		unstake bool
		// withdraw fields
		withdrawIndex uint64
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			identityset.Address(2),
			"9980000000000000000",
			10,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			time.Now(),
			10000,
			true,
			0,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// updateBucket getbucket ErrStateNotExist
		{
			identityset.Address(2),
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			time.Now(),
			10000,
			true,
			1,
			state.ErrStateNotExist,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		},
		// check unstake time
		{
			identityset.Address(2),
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			time.Now(),
			10000,
			false,
			0,
			ErrNotUnstaked,
			iotextypes.ReceiptStatus_ErrWithdrawBeforeUnstake,
		},
		// check ErrNotReadyWithdraw
		{
			identityset.Address(2),
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			time.Now(),
			10000,
			true,
			0,
			ErrNotReadyWithdraw,
			iotextypes.ReceiptStatus_ErrWithdrawBeforeMaturity,
		},
		// nil
		{
			identityset.Address(2),
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			time.Now().Add(time.Hour * 500),
			10000,
			true,
			0,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, _, candidate := initAll(t, ctrl)
		require.NoError(setupAccount(sm, test.caller, test.initBalance))
		ctx, _ := initCreateStake(t, sm, candidate.Owner, test.initBalance, big.NewInt(unit.Qev), test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candidate, test.amount)
		if test.unstake {
			act, err := action.NewUnstake(test.nonce, test.index,
				nil, test.gasLimit, test.gasPrice)
			require.NoError(err)
			intrinsic, err := act.IntrinsicGas()
			require.NoError(err)
			ctx = protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
				Caller:       test.caller,
				GasPrice:     test.gasPrice,
				IntrinsicGas: intrinsic,
				Nonce:        test.nonce,
			})
			ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
				BlockHeight:    1,
				BlockTimeStamp: time.Now(),
				GasLimit:       1000000,
			})

			_, err = p.handleUnstake(ctx, act, sm)
			require.NoError(err)
		}

		withdraw, err := action.NewWithdrawStake(test.nonce, test.withdrawIndex,
			nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		actionCtx := protocol.MustGetActionCtx(ctx)
		blkCtx := protocol.MustGetBlockCtx(ctx)
		ctx = protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       actionCtx.Caller,
			GasPrice:     actionCtx.GasPrice,
			IntrinsicGas: actionCtx.IntrinsicGas,
			Nonce:        actionCtx.Nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    blkCtx.BlockHeight,
			BlockTimeStamp: test.ctxTimestamp,
			GasLimit:       blkCtx.GasLimit,
		})
		r, err := p.handleWithdrawStake(ctx, withdraw, sm)
		if err != nil {
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		}

		if err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			_, err := getCandBucketIndices(sm, candidate.Owner)
			require.Error(err)
			_, err = getVoterBucketIndices(sm, candidate.Owner)
			require.Error(err)
		}

	}
}

func TestProtocol_HandleChangeCandidate(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidate2 := testCandidates[1].d.Clone()
	tests := []struct {
		// creat stake fields
		caller      address.Address
		amount      string
		initBalance int64
		selfstaking bool
		// action fields
		index         uint64
		candidateName string
		gasPrice      *big.Int
		gasLimit      uint64
		nonce         uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// clear flag for inMemCandidates
		clear bool
		// need new p
		newProtocol bool
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// ErrInvalidCanName
		{
			identityset.Address(1),
			"10000000000000000000",
			100,
			false,
			1,
			"testname",
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			true,
			true,
			ErrInvalidCanName,
			iotextypes.ReceiptStatus_ErrCandidateNotExist,
		},
		// fetchCaller state.ErrNotEnoughBalance
		{
			identityset.Address(1),
			"9999990000000000000000",
			10000,
			false,
			1,
			candidate2.Name,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			true,
			true,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// fetchBucket
		{
			identityset.Address(1),
			"10000000000000000000",
			100,
			true,
			1,
			candidate2.Name,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			true,
			ErrFetchBucket,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
		// ErrInvalidOwner
		{
			identityset.Address(1),
			"10000000000000000000",
			100,
			false,
			1,
			candidate2.Name,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			true,
			true,
			ErrInvalidOwner,
			iotextypes.ReceiptStatus_Success,
		},
		// change from 0 to candidate2
		{
			identityset.Address(2),
			"10000000000000000000",
			100,
			false,
			0,
			candidate2.Name,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			false,
			true,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, candidate, candidate2 := initAll(t, ctrl)
		// candidate2 vote self,index 0
		initCreateStake(t, sm, candidate2.Owner, 100, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, "10000000000000000000")
		// candidate vote self,index 1
		initCreateStake(t, sm, candidate.Owner, test.initBalance, test.gasPrice, test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candidate, test.amount)

		act, err := action.NewChangeCandidate(test.nonce, test.candidateName, test.index, nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       1000000,
		})
		if test.clear {
			cc := p.inMemCandidates.GetBySelfStakingIndex(test.index)
			p.inMemCandidates.Delete(cc.Owner)
		}
		r, err := p.handleChangeCandidate(ctx, act, sm)
		if err != nil {
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		}

		if err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, err := getCandBucketIndices(sm, test.caller)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, err = getVoterBucketIndices(sm, test.caller)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := getBucket(sm, indices[0])
			require.NoError(err)
			require.Equal(test.caller.String(), bucket.Candidate.String())
			require.Equal(test.caller.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, err := getCandidate(sm, candidate2.Owner)
			require.NotNil(candidate)
			require.NoError(err)
			require.Equal("10000000000000000003", candidate.Votes.String())
			require.Equal(test.candidateName, candidate.Name)
			require.Equal(candidate2.Operator.String(), candidate.Operator.String())
			require.Equal(candidate2.Reward.String(), candidate.Reward.String())
			require.Equal(candidate2.Owner.String(), candidate.Owner.String())
			require.Equal("1200000000000000000000000", candidate.SelfStake.String())
			candidate = p.inMemCandidates.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.Equal("10000000000000000003", candidate.Votes.String())
		}
	}
}

func TestProtocol_HandleTransferStake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	tests := []struct {
		// creat stake fields
		caller      address.Address
		amount      uint64
		initBalance int64
		// action fields
		index    uint64
		gasPrice *big.Int
		gasLimit uint64
		nonce    uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// NewTransferStake fields
		to            address.Address
		toInitBalance uint64
		init          bool
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller state.ErrNotEnoughBalance
		{
			identityset.Address(2),
			9990000000000000000,
			10,
			0,
			big.NewInt(unit.Qev),
			1000000000,
			1,
			1,
			time.Now(),
			10000,
			identityset.Address(1),
			1,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// fetchBucket,bucket.Owner not equal to actionCtx.Caller
		{
			identityset.Address(1),
			10000000000000000000,
			1000,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			identityset.Address(2),
			1,
			true,
			ErrFetchBucket,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// fetchBucket,inMemCandidates.ContainsSelfStakingBucket is false
		{
			identityset.Address(1),
			10000000000000000000,
			100,
			1,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			identityset.Address(2),
			1,
			true,
			ErrFetchBucket,
			iotextypes.ReceiptStatus_ErrInvalidBucketType,
		},
		{
			identityset.Address(2),
			10000000000000000000,
			100,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			identityset.Address(1),
			1,
			false,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, candi, candidate2 := initAll(t, ctrl)
		_, createCost := initCreateStake(t, sm, candidate2.Owner, test.initBalance, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, fmt.Sprintf("%d", test.amount))
		if test.init {
			initCreateStake(t, sm, candi.Owner, test.initBalance, test.gasPrice, test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candi, fmt.Sprintf("%d", test.amount))
		} else {
			require.NoError(setupAccount(sm, identityset.Address(1), 1))
		}

		act, err := action.NewTransferStake(test.nonce, test.to.String(), test.index, nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)

		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       10000000,
		})
		r, err := p.handleTransferStake(ctx, act, sm)
		if err != nil {
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		}

		if err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, err := getCandBucketIndices(sm, candidate2.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, err = getVoterBucketIndices(sm, test.to)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := getBucket(sm, indices[0])
			require.NoError(err)
			require.Equal(candidate2.Owner, bucket.Candidate)
			require.Equal(test.to.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.Uint64())

			// test candidate
			candidate, err := getCandidate(sm, candi.Owner)
			require.NoError(err)
			require.LessOrEqual(uint64(2), candidate.Votes.Uint64())
			candidate = p.inMemCandidates.GetByOwner(candi.Owner)
			require.NotNil(candidate)
			require.LessOrEqual(uint64(2), candidate.Votes.Uint64())
			require.Equal(candi.Name, candidate.Name)
			require.Equal(candi.Operator, candidate.Operator)
			require.Equal(candi.Reward, candidate.Reward)
			require.Equal(candi.Owner, candidate.Owner)
			require.LessOrEqual(uint64(2), candidate.Votes.Uint64())
			require.LessOrEqual(uint64(2), candidate.SelfStake.Uint64())
			// test staker's account
			caller, err := accountutil.LoadAccount(sm, hash.BytesToHash160(test.caller.Bytes()))
			require.NoError(err)
			actCost, err := act.Cost()
			require.NoError(err)
			require.Equal(test.nonce, caller.Nonce)
			total := big.NewInt(0)
			require.Equal(unit.ConvertIotxToRau(test.initBalance), total.Add(total, caller.Balance).Add(total, actCost).Add(total, createCost))
		}
	}
}

func TestProtocol_HandleRestake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	callerAddr := identityset.Address(2)

	tests := []struct {
		// creat stake fields
		caller      address.Address
		amount      string
		initBalance int64
		selfstaking bool
		// action fields
		index    uint64
		gasPrice *big.Int
		gasLimit uint64
		nonce    uint64
		// block context
		blkHeight    uint64
		blkTimestamp time.Time
		blkGasLimit  uint64
		// restake fields
		duration  uint32
		autoStake bool
		// clear flag for inMemCandidates
		clear bool
		// need new p
		newAccount bool
		// expected result
		err    error
		status iotextypes.ReceiptStatus
	}{
		// fetchCaller ErrNotEnoughBalance
		{
			callerAddr,
			"9990000000000000000",
			10,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_ErrNotEnoughBalance,
		},
		// for bucket.Owner is not equal to actionCtx.Caller
		{
			identityset.Address(12),
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			true,
			ErrFetchBucket,
			iotextypes.ReceiptStatus_ErrUnauthorizedOperator,
		},
		// updateBucket getbucket ErrStateNotExist
		{
			identityset.Address(33),
			"10000000000000000000",
			100,
			false,
			1,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			true,
			state.ErrStateNotExist,
			iotextypes.ReceiptStatus_ErrInvalidBucketIndex,
		},
		// for inMemCandidates.GetByOwner,ErrInvalidOwner
		{
			callerAddr,
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			true,
			false,
			ErrInvalidOwner,
			iotextypes.ReceiptStatus_Success,
		},
		{
			callerAddr,
			"10000000000000000000",
			100,
			false,
			0,
			big.NewInt(unit.Qev),
			10000,
			1,
			1,
			time.Now(),
			10000,
			1,
			true,
			false,
			false,
			nil,
			iotextypes.ReceiptStatus_Success,
		},
	}

	for _, test := range tests {
		sm, p, candidate, candidate2 := initAll(t, ctrl)
		initCreateStake(t, sm, candidate2.Owner, test.initBalance, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, test.amount)

		if test.newAccount {
			require.NoError(setupAccount(sm, test.caller, test.initBalance))
		} else {
			candidate = candidate2
		}

		act, err := action.NewRestake(test.nonce, test.index, test.duration, test.autoStake, nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		if test.clear {
			p.inMemCandidates.Delete(test.caller)
		}
		intrinsic, err := act.IntrinsicGas()
		require.NoError(err)
		ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
			Caller:       test.caller,
			GasPrice:     test.gasPrice,
			IntrinsicGas: intrinsic,
			Nonce:        test.nonce,
		})
		ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
			BlockHeight:    1,
			BlockTimeStamp: time.Now(),
			GasLimit:       10000000,
		})
		r, err := p.handleRestake(ctx, act, sm)
		if err != nil {
			require.Equal(test.err, errors.Cause(err))
		}
		if r != nil {
			require.Equal(uint64(test.status), r.Status)
		}

		if err == nil && test.status == iotextypes.ReceiptStatus_Success {
			// test bucket index and bucket
			bucketIndices, err := getCandBucketIndices(sm, candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			bucketIndices, err = getVoterBucketIndices(sm, candidate.Owner)
			require.NoError(err)
			require.Equal(1, len(*bucketIndices))
			indices := *bucketIndices
			bucket, err := getBucket(sm, indices[0])
			require.NoError(err)
			require.Equal(candidate.Owner.String(), bucket.Candidate.String())
			require.Equal(test.caller.String(), bucket.Owner.String())
			require.Equal(test.amount, bucket.StakedAmount.String())

			// test candidate
			candidate, err = getCandidate(sm, candidate.Owner)
			require.NoError(err)
			require.LessOrEqual(2, candidate.Votes.Uint64())
			candidate = p.inMemCandidates.GetByOwner(candidate.Owner)
			require.NotNil(candidate)
			require.LessOrEqual(2, candidate.Votes.Uint64())
		}

	}
}

func TestProtocol_HandleDepositToStake(t *testing.T) {}

func initCreateStake(t *testing.T, sm protocol.StateManager, callerAddr address.Address, initBalance int64, gasPrice *big.Int, gasLimit uint64, nonce uint64, blkHeight uint64, blkTimestamp time.Time, blkGasLimit uint64, p *Protocol, candidate *Candidate, amount string) (context.Context, *big.Int) {
	require := require.New(t)
	require.NoError(setupAccount(sm, callerAddr, initBalance))
	a, err := action.NewCreateStake(nonce, candidate.Name, amount, 1, false,
		nil, gasLimit, gasPrice)
	require.NoError(err)
	intrinsic, err := a.IntrinsicGas()
	require.NoError(err)
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller:       callerAddr,
		GasPrice:     gasPrice,
		IntrinsicGas: intrinsic,
		Nonce:        nonce,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    blkHeight,
		BlockTimeStamp: blkTimestamp,
		GasLimit:       blkGasLimit,
	})
	_, err = p.handleCreateStake(ctx, a, sm)
	require.NoError(err)
	cost, err := a.Cost()
	require.NoError(err)
	return ctx, cost
}

func initAll(t *testing.T, ctrl *gomock.Controller) (protocol.StateManager, *Protocol, *Candidate, *Candidate) {
	require := require.New(t)
	sm := newMockStateManager(ctrl)
	_, err := sm.PutState(
		&totalBucketCount{count: 0},
		protocol.NamespaceOption(StakingNameSpace),
		protocol.KeyOption(TotalBucketKey),
	)
	require.NoError(err)

	// create protocol
	p, err := NewProtocol(depositGas, sm, genesis.Default.Staking)
	require.NoError(err)

	// set up candidate
	candidate := testCandidates[0].d.Clone()
	require.NoError(setupCandidate(p, sm, candidate))
	candidate2 := testCandidates[1].d.Clone()
	require.NoError(setupCandidate(p, sm, candidate2))
	return sm, p, candidate, candidate2
}

func setupAccount(sm protocol.StateManager, addr address.Address, balance int64) error {
	if balance < 0 {
		return errors.New("balance cannot be negative")
	}
	account, err := accountutil.LoadOrCreateAccount(sm, addr.String())
	if err != nil {
		return err
	}
	account.Balance = unit.ConvertIotxToRau(balance)
	return accountutil.StoreAccount(sm, addr.String(), account)
}

func setupCandidate(p *Protocol, sm protocol.StateManager, candidate *Candidate) error {
	if err := putCandidate(sm, candidate); err != nil {
		return err
	}
	p.inMemCandidates.Upsert(candidate)
	return nil
}

func depositGas(ctx context.Context, sm protocol.StateManager, gasFee *big.Int) error {
	actionCtx := protocol.MustGetActionCtx(ctx)
	// Subtract balance from caller
	acc, err := accountutil.LoadAccount(sm, hash.BytesToHash160(actionCtx.Caller.Bytes()))
	if err != nil {
		return err
	}
	acc.Balance = big.NewInt(0).Sub(acc.Balance, gasFee)
	return accountutil.StoreAccount(sm, actionCtx.Caller.String(), acc)
}
