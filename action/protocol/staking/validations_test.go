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
	"github.com/iotexproject/iotex-core/pkg/unit"
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
	p, candidateName := initTestProtocol(t)
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
			candidateName.Name,
			"1000000000000000000",
			1,
			false,
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidAmount,
		},
		{
			candidateName.Name,
			"200000000000000000000",
			1,
			false,
			big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
		{
			candidateName.Name,
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

	p, candidateName := initTestProtocol(t)

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
			candidateName.Name,
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{"12132323",
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{"~1",
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{"100000000000000000000",
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{candidateName.Name,
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

	p, _ := initTestProtocol(t)
	voterAddress := "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
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
			voterAddress,
			1,
			[]byte("100000000000000000000"),
			big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{voterAddress,
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

	p, can := initTestProtocol(t)
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
			"test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", can.Owner.String(), "1", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			nil,
		},
		{"!te", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "1", uint32(10000), false, []byte("payload"), big.NewInt(unit.Qev),
			10000,
			1,
			ErrInvalidCanName,
		},
		{"test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "1", uint32(10000), false, []byte("payload"), big.NewInt(-unit.Qev),
			10000,
			1,
			action.ErrGasPrice,
		},
	}

	for _, test := range tests {
		act, err := action.NewCandidateRegister(test.nonce, test.name, test.operatorAddrStr, test.rewardAddrStr, test.ownerAddrStr, test.amountStr, test.duration, test.autoStake, test.payload, test.gasLimit, test.gasPrice)
		require.NoError(err)
		require.Equal(test.errorCause, errors.Cause(p.validateCandidateRegister(context.Background(), act)))
	}
	// test nil action
	require.Equal(ErrNilAction, errors.Cause(p.validateCandidateRegister(context.Background(), nil)))
}

func TestProtocol_ValidateCandidateUpdate(t *testing.T) {}

func initTestProtocol(t *testing.T) (*Protocol, *Candidate) {
	require := require.New(t)
	p := NewProtocol(nil, nil, Configuration{
		MinStakeAmount: unit.ConvertIotxToRau(100),
	})
	candidate := testCandidates[0].d.Clone()
	require.NoError(p.inMemCandidates.Upsert(candidate))
	return p, candidate
}
