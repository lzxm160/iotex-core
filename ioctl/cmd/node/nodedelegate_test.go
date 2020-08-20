// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package node

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNodeDelegate(t *testing.T) {
	require := require.New(t)
	for {
		time.Sleep(8)
		err, mess := delegates()
		require.NoError(err)
		for _, m := range mess.Delegates {
			if m.Active && m.Production == 0 {
				fmt.Println(m)
			}
		}
	}
}
