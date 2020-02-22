// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/test/identityset"
)

var (
	index = uint64(10)
)

func TestDepositSignVerify(t *testing.T) {
	require := require.New(t)
	senderKey := identityset.PrivateKey(27)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	ds, err := NewDepositToStake(nonce, index, amount, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(ds).Build()
	h := elp.Hash()
	require.Equal("219483a7309db9f1c41ac3fa0aadecfbdbeb0448b0dfaee54daec4ec178aa9f1", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	require.Equal("080118c0843d22023130c2023d0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611202313018e80720012a077061796c6f6164", hex.EncodeToString(selp.Serialize()))
	hash := selp.Hash()
	require.Equal("a324d56f5b50e86aab27c0c6d33f9699f36d3ed8e27967a56e644f582bbd5e2d", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
func TestDeposit(t *testing.T) {
	require := require.New(t)
	ds, err := NewDepositToStake(nonce, index, amount, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := ds.Serialize()
	require.Equal("080a120231301a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, ds.GasLimit())
	require.Equal(gasprice, ds.GasPrice())
	require.Equal(nonce, ds.Nonce())

	require.Equal(amount, ds.Amount())
	require.Equal(payload, ds.Payload())
	require.Equal(index, ds.BucketIndex())

	gas, err := ds.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := ds.Cost()
	require.NoError(err)
	require.Equal("107010", cost.Text(10))

	proto := ds.Proto()
	ds2 := &DepositToStake{}
	require.NoError(ds2.LoadProto(proto))
	require.Equal(amount, ds2.Amount().Text(10))
	require.Equal(payload, ds2.Payload())
	require.Equal(amount, ds2.Amount())
	require.Equal(payload, ds2.Payload())
	require.Equal(index, ds2.BucketIndex())
}
