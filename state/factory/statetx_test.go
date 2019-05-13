// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package factory

import (
	"testing"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/stretchr/testify/require"
)

func TestGet(t *testing.T) {
	require := require.New(t)
	state := newStateTX(1, nil, nil)
	require.Equal(1, state.Version())
	require.Equal(hash.ZeroHash256, state.RootHash())
	require.Equal(0, state.Height())
	require.Nil(state.GetDB())

}
