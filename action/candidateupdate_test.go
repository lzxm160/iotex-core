// Copyright (c) 2020 IoTeX Foundation
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

func TestCandidateUpdate(t *testing.T) {
	require := require.New(t)
	cr, err := NewCandidateUpdate(nonce, canAddress, canAddress, canAddress, gaslimit, gasprice)
	require.NoError(err)

	ser := cr.Serialize()
	require.Equal("0a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a61", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(gaslimit, cr.GasLimit())
	require.Equal(gasprice, cr.GasPrice())
	require.Equal(nonce, cr.Nonce())

	require.Equal(canAddress, cr.Name())
	require.Equal(canAddress, cr.OperatorAddress().String())
	require.Equal(canAddress, cr.RewardAddress().String())

	gas, err := cr.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cost, err := cr.Cost()
	require.NoError(err)
	require.Equal("100000", cost.Text(10))

	proto := cr.Proto()
	cr2 := &CandidateUpdate{}
	require.NoError(cr2.LoadProto(proto))
	require.Equal(canAddress, cr2.Name())
	require.Equal(canAddress, cr2.OperatorAddress().String())
	require.Equal(canAddress, cr2.RewardAddress().String())
}

func TestCandidateUpdateSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	cr, err := NewCandidateUpdate(nonce, canAddress, canAddress, canAddress, gaslimit, gasprice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cr).Build()
	h := elp.Hash()
	require.Equal("f332644befa8893fbca97d0e23c72fdc52e8af596c17cd8af72f4a90eb664e20", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0acf01080118c0843d22023130fa02c1010a81010a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611202313018e80720012a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a6132077061796c6f6164124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a416290a11eecf59e84f75e3b0485e65d54b482c260c8ecc35cc7d690c02f0d319b377d3bfc63c3e0e009f88efe14c98dce68405f666dd6b634564f53525c09e65a00", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("f662051a104f3be26125316bba8361d51035346502811550f4731d3c9132854c", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
