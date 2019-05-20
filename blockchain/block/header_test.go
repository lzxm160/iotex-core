// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package block

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestHeader(t *testing.T) {
	require := require.New(t)
	ti := time.Now()
	footer := &Header{
		version:          1,
		height:           2,
		timestamp:        ti,
		prevBlockHash:    nil,
		txRoot:           nil,
		deltaStateDigest: nil,
		receiptRoot:      nil,
		blockSig:         nil,
		pubkey:           nil,
	}
	require.Equal(uint32(1), footer.Version())
	require.Equal(uint64(2), footer.Height())
	require.Equal(ti, footer.Timestamp())
	require.Nil(footer.PrevHash())
	require.Nil(footer.TxRoot())
	require.Nil(footer.DeltaStateDigest())
	require.Nil(footer.PublicKey())
	require.Nil(footer.ReceiptRoot())
	require.Nil(footer.HashBlock())
	require.NotNil(footer.BlockHeaderProto())
	require.NotNil(footer.BlockHeaderCoreProto())
}
