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

	"github.com/golang/protobuf/proto"
	"github.com/stretchr/testify/require"
)

var (
	cuNonce           = uint64(20)
	cuName            = "test"
	cuOperatorAddrStr = "io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng"
	cuRewardAddrStr   = "io1juvx5g063eu4ts832nukp4vgcwk2gnc5cu9ayd"
	cuGasLimit        = uint64(200000)
	cuGasPrice        = big.NewInt(2000)
)

func TestCandidateUpdate(t *testing.T) {
	require := require.New(t)
	cu, err := NewCandidateUpdate(cuNonce, cuName, cuOperatorAddrStr, cuRewardAddrStr, cuGasLimit, cuGasPrice)
	require.NoError(err)

	ser := cu.Serialize()
	require.Equal("0a04746573741229696f31636c36726c32657635646661393838716d677a673278346866617a6d7039766e326736366e671a29696f316a757678356730363365753474733833326e756b7034766763776b32676e6335637539617964", hex.EncodeToString(ser))

	require.NoError(err)
	require.Equal(cuGasLimit, cu.GasLimit())
	require.Equal(cuGasPrice, cu.GasPrice())
	require.Equal(cuNonce, cu.Nonce())

	require.Equal(cuName, cu.Name())
	require.Equal(cuOperatorAddrStr, cu.OperatorAddress().String())
	require.Equal(cuRewardAddrStr, cu.RewardAddress().String())

	gas, err := cu.IntrinsicGas()
	require.NoError(err)
	require.Equal(uint64(10000), gas)
	cost, err := cu.Cost()
	require.NoError(err)
	require.Equal("20000000", cost.Text(10))

	proto := cu.Proto()
	cu2 := &CandidateUpdate{}
	require.NoError(cu2.LoadProto(proto))
	require.Equal(cuName, cu2.Name())
	require.Equal(cuOperatorAddrStr, cu2.OperatorAddress().String())
	require.Equal(cuRewardAddrStr, cu2.RewardAddress().String())
}

func TestCandidateUpdateSignVerify(t *testing.T) {
	require := require.New(t)
	require.Equal("cfa6ef757dee2e50351620dca002d32b9c090cfda55fb81f37f1d26b273743f1", senderKey.HexString())
	cu, err := NewCandidateUpdate(cuNonce, cuName, cuOperatorAddrStr, cuRewardAddrStr, cuGasLimit, cuGasPrice)
	require.NoError(err)

	bd := &EnvelopeBuilder{}
	elp := bd.SetGasLimit(gaslimit).
		SetGasPrice(gasprice).
		SetAction(cu).Build()
	h := elp.Hash()
	require.Equal("46209470b666b5fdb7fcc91444316c186e700006cb5a660abe8533f12e0db004", hex.EncodeToString(h[:]))
	// sign
	selp, err := Sign(elp, senderKey)
	require.NoError(err)
	require.NotNil(selp)
	ser, err := proto.Marshal(selp.Proto())
	require.NoError(err)
	require.Equal("0a8f01080118c0843d22023130820381010a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611229696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a611a29696f3178707136326177383575717a72636367397935686e727976386c64326e6b7079636333677a61124104755ce6d8903f6b3793bddb4ea5d3589d637de2d209ae0ea930815c82db564ee8cc448886f639e8a0c7e94e99a5c1335b583c0bc76ef30dd6a1038ed9da8daf331a415297298bd854eede98ec4f43905124cee967e050c7d20d30f37595f689068fd36f7315420c1e550a6e300c2a7ee734268202d4cff86a4b01fbf0a5f35980c98900", hex.EncodeToString(ser))
	hash := selp.Hash()
	require.Equal("69ab00a70f44036627f565ac174467518be505406977fd0836ca5b95266e0ade", hex.EncodeToString(hash[:]))
	// verify signature
	require.NoError(Verify(selp))
}
