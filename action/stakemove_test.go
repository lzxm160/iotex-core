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
	require.Equal("0a1d080118c0843d22023130e20210080a10e807180122077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a41e2e763aed5b1fd1a8601de0f0ae34eb05162e34b0389ae3418eedbf762f64959634a968313a6516dba3a97b34efba4753bbed3a33d409ecbd45ac75007cd8e9101", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("8816e8f784a1fce40b54d1cd172bb6976fd9552f1570c73d1d9fcdc5635424a9", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
