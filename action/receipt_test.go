// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disset epoch rewarded. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"encoding/hex"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestConvert(t *testing.T) {
	require := require.New(t)
	receipt := &Receipt{1, 1, hash.ZeroHash256, 1, "", nil}
	typeReipt := receipt.ConvertToReceiptPb()
	require.NotNil(typeReipt)

	receipt2 := &Receipt{}
	receipt2.ConvertFromReceiptPb(typeReipt)
	require.Equal(receipt.Status, receipt2.Status)
	require.Equal(receipt.BlockHeight, receipt2.BlockHeight)
	require.Equal(receipt.ActionHash, receipt2.ActionHash)
	require.Equal(receipt.GasConsumed, receipt2.GasConsumed)
	require.Equal(receipt.ContractAddress, receipt2.ContractAddress)
}
func TestSerDer(t *testing.T) {
	require := require.New(t)
	receipt := &Receipt{1, 1, hash.ZeroHash256, 1, "", nil}
	ser, err := receipt.Serialize()
	require.NoError(err)

	receipt2 := &Receipt{}
	receipt2.Deserialize(ser)
	require.Equal(receipt.Status, receipt2.Status)
	require.Equal(receipt.BlockHeight, receipt2.BlockHeight)
	require.Equal(receipt.ActionHash, receipt2.ActionHash)
	require.Equal(receipt.GasConsumed, receipt2.GasConsumed)
	require.Equal(receipt.ContractAddress, receipt2.ContractAddress)

	hash := receipt.Hash()
	require.Equal("", hex.EncodeToString(hash[:]))
}
