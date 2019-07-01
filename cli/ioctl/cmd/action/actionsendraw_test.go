// Copyright (c) 2019 IoTeX
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

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/gogo/protobuf/proto"

	"github.com/iotexproject/go-pkgs/crypto"

	"github.com/iotexproject/iotex-core/action"

	"github.com/stretchr/testify/require"
)

func TestSendRaw(t *testing.T) {
	require := require.New(t)
	nonce := uint64(29)
	amount := big.NewInt(1000000000000000000)
	receipt := "io1eyn9tc6t782zx4zgy3hgt32hpz6t8v7pgf524z"
	gaslimit := uint64(10000)
	gasprice := big.NewInt(0)
	tx, err := action.NewTransfer(nonce, amount,
		receipt, nil, gaslimit, gasprice)
	require.NoError(err)
	elp := (&action.EnvelopeBuilder{}).
		SetNonce(nonce).
		SetGasPrice(gasprice).
		SetGasLimit(gaslimit).
		SetAction(tx).Build()
	pri, err := crypto.HexStringToPrivateKey("0d4d9b248110257c575ef2e8d93dd53471d9178984482817dcbd6edb607f8cc5")
	require.NoError(err)
	sealed, err := action.Sign(elp, pri)
	act := sealed.Proto()
	act.Signature[64] = act.Signature[64] + 27
	b, err := proto.Marshal(act)
	require.NoError(err)
	fmt.Println(hex.EncodeToString(b))

	actBytes, err := hex.DecodeString(hex.EncodeToString(b))
	require.NoError(err)
	actRet := &iotextypes.Action{}
	err = proto.Unmarshal(actBytes, actRet)
	require.NoError(err)
	require.Error(sendRaw(actRet)) //connect error
}
