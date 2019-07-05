// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/blockchain/block"

	"github.com/stretchr/testify/require"
)

func TestNewBlockProposal(t *testing.T) {
	require := require.New(t)
	bp := newBlockProposal(nil, nil)
	require.NotNil(bp)
	require.Panics(func() { bp.Height() }, "block is nil")
	require.Panics(func() { bp.Proto() }, "block is nil")
	require.Panics(func() { bp.Hash() }, "block is nil")
	require.Panics(func() { bp.ProposerAddress() }, "block is nil")

	hcore := &iotextypes.BlockHeaderCore{Height: 123}
	header := &block.Header{}
	header.LoadFromBlockHeaderProto(&iotextypes.BlockHeader{Core: hcore})
	b := block.Block{Header: *header}
	bp2 := newBlockProposal(&b, nil)
	require.NotNil(bp2)
	require.Equal(uint64(123), bp2.Height())
}
