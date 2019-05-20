// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/stretchr/testify/require"
)

func TestHeader(t *testing.T) {
	require := require.New(t)
	ti := time.Now()
	footer := &Header{
		version:          1,
		height:           2,
		timestamp:        ti,
		prevBlockHash:    hash.Hash256b([]byte("")),
		txRoot:           hash.Hash256b([]byte("")),
		deltaStateDigest: hash.Hash256b([]byte("")),
		receiptRoot:      hash.Hash256b([]byte("")),
		blockSig:         nil,
		pubkey:           nil,
	}
	require.Equal(uint32(1), footer.Version())
	require.Equal(uint64(2), footer.Height())
	require.Equal(ti, footer.Timestamp())
	expected := "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	require.True(isEqual(expected, footer.PrevHash()))
	require.True(isEqual(expected, footer.TxRoot()))
	require.True(isEqual(expected, footer.DeltaStateDigest()))
	require.Nil(footer.PublicKey())
	require.True(isEqual(expected, footer.ReceiptRoot()))
	require.True(isEqual(expected, footer.HashBlock()))
	require.NotNil(footer.BlockHeaderProto())
	require.NotNil(footer.BlockHeaderCoreProto())
}
func isEqual(expected string, hash hash.Hash256) bool {
	h := fmt.Sprintf("%x", hash[:])
	fmt.Println("expected:", expected)
	fmt.Println("hash:", h)
	return strings.EqualFold(expected, h)
}
