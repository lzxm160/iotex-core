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

func TestCreateStakeSignVerify(t *testing.T) {
	require := require.New(t)
	recipientAddr := identityset.Address(28)
	senderKey := identityset.PrivateKey(27)

	tsf, err := NewTransfer(0, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)

	tsf.Proto()

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()

	elp.Serialize()

	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))

	// sign the transfer
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)

	// verify signature
	require.NoError(Verify(selp))
}
func TestCreateStake(t *testing.T) {
	require := require.New(t)
	senderKey := identityset.PrivateKey(27)

	gaslimit := uint64(1000000)
	gasprice := big.NewInt(10)
	canName := "io1xpq62aw85uqzrccg9y5hnryv8ld2nkpycc3gza"
	cs, err := NewCreateStake(0, canName, "10", 1000, true, []byte("payload"), gaslimit, gasprice)
	require.NoError(err)

	ser := cs.Serialize()
	fmt.Println("CreateStake ser:", hex.EncodeToString(ser))
	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cs).Build()

	ser = elp.Serialize()
	fmt.Println("CreateStake elp ser:", hex.EncodeToString(ser))
	w := AssembleSealedEnvelope(elp, senderKey.PublicKey(), []byte("lol"))
	require.Error(Verify(w))

	require.NoError(err)
	require.Equal("10", cs.Amount().Text(10))
	require.Equal([]byte("payload"), cs.Payload())
	require.Equal(gaslimit, cs.GasLimit())
	require.Equal("10", cs.GasPrice().Text(10))
	require.Equal(uint64(0), cs.Nonce())

	require.Equal(canName, cs.Candidate())
	require.Equal(uint32(1000), cs.Duration())
	require.True(cs.AutoStake())

	gas, err := cs.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cost, err := cs.Cost()
	require.NoError(err)
	require.Equal("100010", cost.Text(10))

	proto := cs.Proto()
	cs2 := &CreateStake{}
	require.NoError(cs2.LoadProto(proto))
	require.Equal("10", cs2.Amount().Text(10))
	require.Equal([]byte("payload"), cs2.Payload())
	require.Equal(gaslimit, cs2.GasLimit())
	require.Equal("10", cs2.GasPrice().Text(10))
	require.Equal(uint64(0), cs2.Nonce())

	require.Equal(canName, cs2.Candidate())
	require.Equal(1000, cs2.Duration())
	require.True(cs2.AutoStake())
}
