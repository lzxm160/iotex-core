// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	gaslimit  = uint64(1000000)
	gasprice  = big.NewInt(10)
	canName   = "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
	payload   = []byte("payload")
	amount    = "10"
	nonce     = uint64(0)
	duration  = uint32(1000)
	autoStake = true
)

func TestCreateStakeSignVerify(t *testing.T) {
	require := require.New(t)
	senderKey := identityset.PrivateKey(27)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	cs, err := NewCreateStake(nonce, canName, amount, duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cs).Build()
	h := elp.Hash()
	require.Equal("219483a7309db9f1c41ac3fa0aadecfbdbeb0448b0dfaee54daec4ec178aa9f1", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	require.Equal("080118c0843d22023130c2023d0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611202313018e80720012a077061796c6f6164", hex.EncodeToString(selp.Serialize()))
	require.Equal("080118c0843d22023130c2023d0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611202313018e80720012a077061796c6f6164", hex.EncodeToString(selp.Hash()[:]))
	// verify signature
	require.NoError(Verify(selp))
}
func TestCreateStake(t *testing.T) {
	require := require.New(t)
	cs, err := NewCreateStake(nonce, canName, amount, duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := cs.Serialize()
	require.Equal("0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611202313018e80720012a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, cs.GasLimit())
	require.Equal(gasprice, cs.GasPrice())
	require.Equal(nonce, cs.Nonce())

	require.Equal(amount, cs.Amount().Text(10))
	require.Equal(payload, cs.Payload())
	require.Equal(canName, cs.Candidate())
	require.Equal(duration, cs.Duration())
	require.True(cs.AutoStake())

	gas, err := cs.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := cs.Cost()
	require.NoError(err)
	require.Equal("107010", cost.Text(10))

	proto := cs.Proto()
	cs2 := &CreateStake{}
	require.NoError(cs2.LoadProto(proto))
	require.Equal(amount, cs2.Amount().Text(10))
	require.Equal(payload, cs2.Payload())
	require.Equal(canName, cs2.Candidate())
	require.Equal(duration, cs2.Duration())
	require.True(cs2.AutoStake())
}
