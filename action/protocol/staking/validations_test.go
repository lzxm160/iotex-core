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

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/pkg/unit"
	"github.com/iotexproject/iotex-core/test/identityset"
)

func TestIsValidCandidateName(t *testing.T) {
	tests := []struct {
		input  string
		output bool
	}{
		{
			input:  "abc",
			output: true,
		},
		{
			input:  "123",
			output: true,
		},
		{
			input:  "abc123abc123",
			output: true,
		},
		{
			input:  "Abc123",
			output: false,
		},
		{
			input:  "Abc 123",
			output: false,
		},
		{
			input:  "Abc-123",
			output: false,
		},
		{
			input:  "abc123abc123abc123",
			output: false,
		},
		{
			input:  "",
			output: false,
		},
	}

	for _, tt := range tests {
		output := IsValidCandidateName(tt.input)
		assert.Equal(t, tt.output, output)
	}
}

func TestProtocol_ValidateCreateStake(t *testing.T) {
	require := require.New(t)
	p, cands := initTestProtocol(t)
	tests := []struct {
		// action fields
		candName  string
		amount    string
		duration  uint32
		autoStake bool
		gasPrice  *big.Int
		gasLimit  uint64
		nonce     uint64
		// expected results
		errorCause error
	}{
		{
			"",
			"100000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{
			"$$$",
			"100000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{
			"123",
			"200000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{
			cands[0].Name,
			"1000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidAmount,
		},
		{
			cands[0].Name,
			"200000000000000000000",
			1,
			false,
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
		{
			cands[0].Name,
			"200000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
	}

	for _, test := range tests {
		act, err := action.NewCreateStake(test.nonce, test.candName, test.amount, test.duration, test.autoStake,
			nil, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateCreateStake(context.Background(), act)))
	}
}

func TestProtocol_ValidateUnstake(t *testing.T) {
	require := require.New(t)

	p, _ := initTestProtocol(t)

	tests := []struct {
		bucketIndex uint64
		payload     []byte
		gasPrice    *big.Int
		gasLimit    uint64
		nonce       uint64
		// expected results
		errorCause error
	}{
		{
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{1,
			[]byte("100000000000000000000"),
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewUnstake(test.nonce, test.bucketIndex, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateUnstake(context.Background(), act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateUnstake(context.Background(), nil)))
}

func TestProtocol_ValidateWithdrawStake(t *testing.T) {
	require := require.New(t)

	p, _ := initTestProtocol(t)

	tests := []struct {
		bucketIndex uint64
		payload     []byte
		gasPrice    *big.Int
		gasLimit    uint64
		nonce       uint64
		// expected results
		errorCause error
	}{
		{
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{1,
			[]byte("100000000000000000000"),
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewWithdrawStake(test.nonce, test.bucketIndex, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateWithdrawStake(context.Background(), act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateWithdrawStake(context.Background(), nil)))
}

func TestProtocol_ValidateChangeCandidate(t *testing.T) {
	require := require.New(t)

	p, cands := initTestProtocol(t)

	tests := []struct {
		candName    string
		bucketIndex uint64
		payload     []byte
		gasPrice    *big.Int
		gasLimit    uint64
		nonce       uint64
		// expected results
		errorCause error
	}{
		{
			cands[0].Name,
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		// inMemCandidates not contain
		{"12132323",
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// IsValidCandidateName special char
		{"~1",
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// IsValidCandidateName len>12
		{"100000000000000000000",
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// IsValidCandidateName len==0
		{"",
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{cands[0].Name,
			1,
			[]byte("100000000000000000000"),
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewChangeCandidate(test.nonce, test.candName, test.bucketIndex, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateChangeCandidate(context.Background(), act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateChangeCandidate(context.Background(), nil)))
}

func TestProtocol_ValidateTransferStake(t *testing.T) {
	require := require.New(t)

	p, cans := initTestProtocol(t)
	tests := []struct {
		voterAddress string
		bucketIndex  uint64
		payload      []byte
		gasPrice     *big.Int
		gasLimit     uint64
		nonce        uint64
		// expected results
		errorCause error
	}{
		{
			cans[0].Operator.String(),
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{cans[0].Operator.String(),
			1,
			[]byte("100000000000000000000"),
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewTransferStake(test.nonce, test.voterAddress, test.bucketIndex, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateTransferStake(context.Background(), act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateTransferStake(context.Background(), nil)))
}

func TestProtocol_ValidateDepositToStake(t *testing.T) {
	require := require.New(t)

	p, _ := initTestProtocol(t)
	tests := []struct {
		index    uint64
		amount   string
		payload  []byte
		gasPrice *big.Int
		gasLimit uint64
		nonce    uint64
		// expected results
		errorCause error
	}{
		{
			1,
			"10",
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{1,
			"10",
			[]byte("100000000000000000000"),
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewDepositToStake(test.nonce, test.index, test.amount, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateDepositToStake(context.Background(), act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateDepositToStake(context.Background(), nil)))
}

func TestProtocol_ValidateRestake(t *testing.T) {
	require := require.New(t)
	p, _ := initTestProtocol(t)
	tests := []struct {
		index     uint64
		duration  uint32
		autoStake bool
		payload   []byte
		gasPrice  *big.Int
		gasLimit  uint64
		nonce     uint64
		// expected results
		errorCause error
	}{
		{
			1,
			10,
			true,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{1,
			10,
			true,
			[]byte("100000000000000000000"),
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewRestake(test.nonce, test.index, test.duration, test.autoStake, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateRestake(context.Background(), act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateRestake(context.Background(), nil)))
}

func TestProtocol_ValidateCandidateRegister(t *testing.T) {
	require := require.New(t)

	p, cans := initTestProtocol(t)
	tests := []struct {
		name            string
		operatorAddrStr string
		rewardAddrStr   string
		ownerAddrStr    string
		amountStr       string
		duration        uint32
		autoStake       bool
		payload         []byte
		gasPrice        *big.Int
		gasLimit        uint64
		nonce           uint64
		// expected results
		errorCause error
	}{
		{
			"test1", cans[0].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		// an existing owner, but selfstake is not 0
		{
			"test2", cans[1].Operator.String(), cans[1].Reward.String(), cans[1].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidOwner,
		},
		// IsValidCandidateName special char
		{"!te", cans[0].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// IsValidCandidateName len>12
		{"100000000000000000000", cans[0].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// IsValidCandidateName len==0
		{"", cans[0].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{"test1", cans[0].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
		// amount<minSelfStake
		{
			"test1", cans[0].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "1", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidAmount,
		},
		// act.Name() != c.Name && p.inMemCandidates.ContainsName(act.Name())
		{
			"test", cans[0].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// !address.Equal(act.OperatorAddress(), c.Operator) && p.inMemCandidates.ContainsOperator(act.OperatorAddress())
		{
			"test1", cans[1].Operator.String(), cans[0].Reward.String(), cans[0].Owner.String(), "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidOperator,
		},
		// actCtx.Caller and act.OwnerAddress() is "",p.inMemCandidates.GetByOwner(owner) returns nil，p.inMemCandidates.ContainsName(act.Name())
		{
			"test1", cans[0].Operator.String(), cans[0].Reward.String(), "", "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// actCtx.Caller and act.OwnerAddress() is "",p.inMemCandidates.GetByOwner(owner) returns nil，p.inMemCandidates.ContainsOperator(act.OperatorAddress())
		{
			"2222", cans[0].Operator.String(), cans[0].Reward.String(), "", "100000000000000000000", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidOperator,
		},
	}

	ctx := protocol.WithActionCtx(
		context.Background(),
		protocol.ActionCtx{},
	)
	for _, test := range tests {
		act, err := action.NewCandidateRegister(test.nonce, test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateCandidateRegister(ctx, act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateCandidateRegister(ctx, nil)))
}

func TestProtocol_ValidateCandidateUpdate(t *testing.T) {
	require := require.New(t)
	p, cans := initTestProtocol(t)
	ctx := protocol.WithActionCtx(
		context.Background(),
		protocol.ActionCtx{},
	)
	ctx2 := protocol.WithActionCtx(
		context.Background(),
		protocol.ActionCtx{Caller: cans[0].Owner},
	)
	tests := []struct {
		ctx             context.Context
		name            string
		operatorAddrStr string
		rewardAddrStr   string
		gasPrice        *big.Int
		gasLimit        uint64
		nonce           uint64
		// expected results
		errorCause error
	}{
		{
			ctx2, "test1", cans[0].Operator.String(), cans[0].Reward.String(), big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		// IsValidCandidateName special char
		{ctx2, "!te", cans[0].Operator.String(), cans[0].Reward.String(), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// IsValidCandidateName len>12
		{ctx2, "100000000000000", cans[0].Operator.String(), cans[0].Reward.String(), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		// IsValidCandidateName len==0
		{ctx2, "", cans[0].Operator.String(), cans[0].Reward.String(), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{
			ctx, "test", cans[1].Operator.String(), cans[1].Reward.String(), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidOwner,
		},
		// len(act.Name()) != 0 && act.Name() != c.Name && p.inMemCandidates.ContainsName(act.Name()
		{ctx2, "test", cans[0].Operator.String(), cans[0].Reward.String(), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{ctx2, "test", cans[1].Operator.String(), cans[0].Reward.String(), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidOperator,
		},
		// ErrGasPrice
		{ctx2, "test1", cans[0].Operator.String(), cans[0].Reward.String(), big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewCandidateUpdate(test.nonce, test.name, test.operatorAddrStr, test.rewardAddrStr, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateCandidateUpdate(test.ctx, act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateCandidateUpdate(ctx, nil)))
}

func initTestProtocol(t *testing.T) (*Protocol, []*Candidate) {
	require := require.New(t)
	p := NewProtocol(nil, nil, Configuration{
		MinStakeAmount: 100,
		Register:       RegistrationConsts{MinSelfStake: 100},
	})
	var cans []*Candidate
	cans = append(cans, &Candidate{
		Owner:              identityset.Address(1),
		Operator:           identityset.Address(11),
		Reward:             identityset.Address(1),
		Name:               "test1",
		Votes:              big.NewInt(2),
		SelfStakeBucketIdx: 1,
		SelfStake:          big.NewInt(0),
	})
	cans = append(cans, &Candidate{
		Owner:              identityset.Address(28),
		Operator:           identityset.Address(28),
		Reward:             identityset.Address(29),
		Name:               "test2",
		Votes:              big.NewInt(2),
		SelfStakeBucketIdx: 2,
		SelfStake:          big.NewInt(10),
	})
	cans = append(cans, &Candidate{
		Owner:              identityset.Address(28),
		Operator:           identityset.Address(28),
		Reward:             identityset.Address(29),
		Name:               "test",
		Votes:              big.NewInt(2),
		SelfStakeBucketIdx: 2,
		SelfStake:          big.NewInt(10),
	})
	for _, can := range cans {
		require.NoError(p.inMemCandidates.Upsert(can))
	}

	return p, cans
}
