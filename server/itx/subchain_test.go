// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package itx

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/stretchr/testify/assert"
)

func TestGetSubChainDBPath(t *testing.T) {
	t.Parallel()

	chainDBPath := getSubChainDBPath(1, config.Default.Chain.ChainDBPath)
	trieDBPath := getSubChainDBPath(1, config.Default.Chain.TrieDBPath)
	assert.Equal(t, "chain-1-chain.db", chainDBPath)
	assert.Equal(t, "chain-1-trie.db", trieDBPath)
}
func TestHandleBlock(t *testing.T) {
	require := require.New(t)
	cfg, err := config.New()
	require.NoError(err)
	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Genesis.EnableGravityChainVoting = true
	ss, err := NewServer(cfg)
	require.NoError(err)
	require.NotNil(ss)
	ctx, cancel := context.WithCancel(context.Background())
	livenessCtx, livenessCancel := context.WithCancel(context.Background())
	probeSvr := probe.New(cfg.System.HTTPStatsPort)
	err = probeSvr.Start(ctx)
	require.NoError(err)
	go StartServer(ctx, ss, probeSvr, cfg)

	err = ss.HandleBlock(nil)
	require.Error(err)
	time.Sleep(time.Second * 2)
	cancel()
	err = probeSvr.Stop(livenessCtx)
	require.NoError(err)
	livenessCancel()
}
