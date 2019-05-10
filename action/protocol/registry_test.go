// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRegister(t *testing.T) {
	require := require.New(t)
	reg := &Registry{}
	// Case I: Normal
	{
		require.NoError(reg.Register("1", nil))
	}
	// Case II: GasLimit higher
	{
		require.NoError(reg.Register("1", nil))
		require.Error(reg.Register("1", nil))
	}
	// Case III: GasLimit lower
	{
	}
	// Case IV: Call cm Nonce err
	{

	}
	// Case V: Call Nonce err
	{

	}
}
