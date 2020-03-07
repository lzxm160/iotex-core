// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/pkg/errors"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
)

var testParams = []struct {
	Nonce           uint64
	Name            string
	OperatorAddrStr string
	RewardAddrStr   string
	OwnerAddrStr    string
	AmountStr       string
	Duration        uint32
	AutoStake       bool
	Payload         []byte
	GasLimit        uint64
	GasPrice        *big.Int
	Serialize       string
	IntrinsicGas    uint64
	Cost            string
	ElpHash         string
	Sign            string
	SelpHash        string
	Expected        error
}{
	// valid test
	{
		uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "100", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "0a5c0a04746573741229696f3130613239387a6d7a7672743467757137396139663478377165646a35397937657279383468651a29696f3133736a396d7a7065776e3235796d6865756b74653476333968766a647472667030306d6c7976120331303018904e2a29696f313964307033616834673877773964376b63786671383779786537666e7238727074683573686a32077061796c6f6164", uint64(10700), "10700100", "769725930ed38023058bb4f01c220feef2e3e40febb36856dfb780c4f7b1ea9b", "0aa801080118c0843d22023130fa029a010a5c0a04746573741229696f3130613239387a6d7a7672743467757137396139663478377165646a35397937657279383468651a29696f3133736a396d7a7065776e3235796d6865756b74653476333968766a647472667030306d6c7976120331303018904e2a29696f313964307033616834673877773964376b63786671383779786537666e7238727074683573686a32077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41a24fbde43c0bee6f3ce703f449fcb618b84b52a66eb294c89bd13ec1eaece39d39010ec28297f7c299185852ff85af77baa134dd963622d53c6eeee1bd56b22700", "5d02b48f36d1b862e527fbd931a3c736c9434edd98e438aa3da8199c029a546f", nil,
	},
	// invalid test
	{
		uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "-10", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", ErrInvalidAmount,
	},
	{
		uint64(10), "test", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "0", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000), "", uint64(10700), "", "", "", "", ErrInvalidAmount,
	},
}

func TestCandidateRegister(t *testing.T) {
	require := require.New(t)
	for _, test := range testParams {
		cr, err := NewCandidateRegister(test.Nonce, test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.Payload, test.GasLimit, test.GasPrice)
		require.Equal(test.Expected, errors.Cause(err))

		if err != nil {
			continue
		}

		require.Equal(test.Serialize, hex.EncodeToString(cr.Serialize()))

		require.NoError(err)
		require.Equal(test.GasLimit, cr.GasLimit())
		require.Equal(test.GasPrice, cr.GasPrice())
		require.Equal(test.Nonce, cr.Nonce())

		require.Equal(test.Name, cr.Name())
		require.Equal(test.OperatorAddrStr, cr.OperatorAddress().String())
		require.Equal(test.RewardAddrStr, cr.RewardAddress().String())
		require.Equal(test.OwnerAddrStr, cr.OwnerAddress().String())
		require.Equal(test.AmountStr, cr.Amount().String())
		require.Equal(test.Duration, cr.Duration())
		require.Equal(test.AutoStake, cr.AutoStake())
		require.Equal(test.Payload, cr.Payload())

		gas, err := cr.IntrinsicGas()
		require.NoError(err)
		require.Equal(test.IntrinsicGas, gas)
		cost, err := cr.Cost()
		require.NoError(err)
		require.Equal(test.Cost, cost.Text(10))

		proto := cr.Proto()
		cr2 := &CandidateRegister{}
		require.NoError(cr2.LoadProto(proto))
		require.Equal(test.Name, cr2.Name())
		require.Equal(test.OperatorAddrStr, cr2.OperatorAddress().String())
		require.Equal(test.RewardAddrStr, cr2.RewardAddress().String())
		require.Equal(test.OwnerAddrStr, cr2.OwnerAddress().String())
		require.Equal(test.AmountStr, cr2.Amount().String())
		require.Equal(test.Duration, cr2.Duration())
		require.Equal(test.AutoStake, cr2.AutoStake())
		require.Equal(test.Payload, cr2.Payload())
	}
}

func TestCandidateRegisterSignVerify(t *testing.T) {
	require := require.New(t)
	for _, test := range testParams {
		cr, err := NewCandidateRegister(test.Nonce, test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.Payload, test.GasLimit, test.GasPrice)
		require.Equal(test.Expected, errors.Cause(err))

		if err != nil {
			continue
		}

		bd := &EnvelopeBuilder{}
		elp := bd.SetGasLimit(test.GasLimit).
			SetGasPrice(test.GasPrice).
			SetAction(cr).Build()
		h := elp.Hash()
		require.Equal(test.ElpHash, hex.EncodeToString(h[:]))
		// sign
		selp, err := Sign(elp, senderKey)
		require.NoError(err)
		require.NotNil(selp)
		ser, err := proto.Marshal(selp.Proto())
		require.NoError(err)
		require.Equal(test.Sign, hex.EncodeToString(ser))
		hash := selp.Hash()
		require.Equal(test.SelpHash, hex.EncodeToString(hash[:]))
		// verify signature
		require.NoError(Verify(selp))
	}

}
