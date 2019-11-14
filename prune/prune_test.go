// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package api

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/config"
	"github.com/stretchr/testify/require"
)

func TestNewPrune(t *testing.T) {
	require := require.New(t)
	p := NewPrune(config.Default)
	require.NoError(p.Start(context.Background()))
	time.Sleep(20)
	require.NoError(p.Stop(context.Background()))
}
