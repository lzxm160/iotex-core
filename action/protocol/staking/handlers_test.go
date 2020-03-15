// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-address/address"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	accountutil "github.com/iotexproject/iotex-core/action/protocol/account/util"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestProtocol_HandleCreateStake(t *testing.T) {
	require := require.New(t)

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	sm, p, candidate, _ := initAll(t, ctrl)
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
		errorCause error
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
			state.ErrNotEnoughBalance,
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
			ErrInvalidCanName,
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
			nil,
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
		_, err = p.handleCreateStake(ctx, act, sm)
		require.Equal(test.errorCause, errors.Cause(err))

		if test.errorCause == nil {
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

func TestProtocol_HandleUnstake(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm, p, candidate, candidate2 := initAll(t, ctrl)
	ctx := initCreateStake(t, sm, identityset.Address(2), 100, big.NewInt(unit.Qev), 10000, 1, 1, time.Now(), 10000, p, candidate2, "10000000000000000000")

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
		// expected result
		errorCause error
	}{
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
		},
		// test fetchCaller error ErrNotEnoughBalance
		// 9990000000000000000+gas(10000000000000000)=10 iotx,no more extra balance
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
			state.ErrNotEnoughBalance,
		},
		// getbucket ErrStateNotExist
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
		},
		// for bucket.Owner is not equal to actionCtx.Caller
		{
			identityset.Address(11),
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
		},
		// failed to subtract vote for candidate
		//{
		//	callerAddr,
		//	"9980000000000000000",
		//	10,
		//	true,
		//	0,
		//	big.NewInt(unit.Qev),
		//	10000,
		//	1,
		//	1,
		//	time.Now(),
		//	10000,
		//	false,
		//	ErrInvalidAmount,
		//},
		// for inMemCandidates.GetByOwner,have to put in the bottom
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
		},
	}

	for _, test := range tests {
		if test.newProtocol {
			sm, p, candidate, _ = initAll(t, ctrl)
			ctx = initCreateStake(t, sm, test.caller, test.initBalance, test.gasPrice, test.gasLimit, test.nonce, test.blkHeight, test.blkTimestamp, test.blkGasLimit, p, candidate, test.amount)
		} else {
			candidate = candidate2
		}

		act, err := action.NewUnstake(test.nonce, test.index,
			nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		if test.clear {
			p.inMemCandidates.Delete(test.caller)
		}
		_, err = p.handleUnstake(ctx, act, sm)
		require.Equal(test.errorCause, errors.Cause(err))

		if test.errorCause == nil {
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

func initCreateStake(t *testing.T, sm protocol.StateManager, callerAddr address.Address, initBalance int64, gasPrice *big.Int, gasLimit uint64, nonce uint64, blkHeight uint64, blkTimestamp time.Time, blkGasLimit uint64, p *Protocol, candidate *Candidate, amount string) context.Context {
	require := require.New(t)
	require.NoError(setupAccount(sm, callerAddr, initBalance))
	ctx := protocol.WithActionCtx(context.Background(), protocol.ActionCtx{
		Caller:       callerAddr,
		GasPrice:     gasPrice,
		IntrinsicGas: gasLimit,
		Nonce:        nonce,
	})
	ctx = protocol.WithBlockCtx(ctx, protocol.BlockCtx{
		BlockHeight:    blkHeight,
		BlockTimeStamp: blkTimestamp,
		GasLimit:       blkGasLimit,
	})
	a, err := action.NewCreateStake(nonce, candidate.Name, amount, 1, false,
		nil, gasLimit, gasPrice)
	require.NoError(err)
	_, err = p.handleCreateStake(ctx, a, sm)
	require.NoError(err)
	return ctx
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
