// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
)

func TestNewServer(t *testing.T) {
	require := require.New(t)
	s, err := NewServer(config.Default)
	require.NoError(err)
	require.NotNil(s)
}
func TestNewInMemTestServer(t *testing.T) {
	require := require.New(t)
	s, err := NewInMemTestServer(config.Default)
	require.NoError(err)
	require.NotNil(s)
}
func TestStartStop(t *testing.T) {
	require := require.New(t)
	ctx := context.Background()
	s, err := NewServer(config.Default)
	require.NoError(err)
	require.NotNil(s)
	err = s.Start(ctx)
	require.NoError(err)
	err = s.Stop(ctx)
	require.NoError(err)
	err = s.NewSubChainService(config.Default)
	require.NoError(err)
	err = s.StopChainService(ctx, 0)
	require.Error(err)

	ss, err := NewServer(config.Default)
	require.NoError(err)
	require.NotNil(ss)
	err = ss.StopChainService(ctx, 1)
	require.NoError(err)
}
