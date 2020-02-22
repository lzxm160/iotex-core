// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"fmt"
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
	fmt.Println("pri:", senderKey.HexString())

	cs, err := NewCreateStake(nonce, canName, amount, duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cs).Build()
	h := elp.Hash()
	fmt.Println("hash:", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	fmt.Println("selp:", hex.EncodeToString(selp.Serialize()))
	// verify signature
	require.NoError(Verify(selp))
}
func TestCreateStake(t *testing.T) {
	require := require.New(t)
	cs, err := NewCreateStake(nonce, canName, amount, duration, autoStake, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := cs.Serialize()
	fmt.Println("CreateStake ser:", hex.EncodeToString(ser))

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
