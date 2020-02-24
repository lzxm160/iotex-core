// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
)

func TestChangeCandidate(t *testing.T) {
	require := require.New(t)
	stake, err := NewChangeCandidate(nonce, canName, index, payload, gaslimit, gasprice)
	require.NoError(err)

	ser := stake.Serialize()
	require.Equal("080a1a077061796c6f6164", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, stake.GasLimit())
	require.Equal(gasprice, stake.GasPrice())
	require.Equal(nonce, stake.Nonce())

	require.Equal(payload, stake.Payload())
	require.Equal(canName, stake.Name())
	require.Equal(index, stake.BucketIndex())

	gas, err := stake.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10700), gas)
	cost, err := stake.Cost()
	require.NoError(err)
	require.Equal("107000", cost.Text(10))

	proto := stake.Proto()
	stake2 := &ChangeCandidate{}
	require.NoError(stake2.LoadProto(proto))
	require.Equal(payload, stake2.Payload())
	require.Equal("", stake2.Name())
	require.Equal(index, stake2.BucketIndex())
}

func TestChangeCandidateSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	stake, err := NewChangeCandidate(nonce, canName, index, payload, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(stake).Build()
	h := elp.Hash()
	require.Equal("58258bd01d7b7e2500f79126feeffec8642ddcc9d6a7c275c144ba8b1c8d6c81", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a18080118c0843d22023130ea020b080a1a077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a412b801345168f97445ed6f86555878451b8d7da09f72814c4159fe571f81aa7310eebfa17a1b3263b42f102861d485aea91424801a91c678e35527b3a19e16cf201", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("06a692dee28596e28aa0fe2f7eb65a141d25dde7d1451b4eb529a25fe0572a79", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
