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
	s, err := NewServer(config.Default)
	require.NoError(err)
	require.NotNil(s)
	err = s.Start(context.Background())
	require.NoError(err)
	err = s.Stop(context.Background())
	require.NoError(err)
}
