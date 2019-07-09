// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/config"
	"github.com/stretchr/testify/require"
)

func TestRollDPoSCtx(t *testing.T) {
	//newRollDPoSCtx(
	//	cfg config.RollDPoS,
	//	active bool,
	//	blockInterval time.Duration,
	//	toleratedOvertime time.Duration,
	//	timeBasedRotation bool,
	//	chain blockchain.Blockchain,
	//	actPool actpool.ActPool,
	//	rp *rolldpos.Protocol,
	//	broadcastHandler scheme.Broadcast,
	//	candidatesByHeightFunc CandidatesByHeightFunc,
	//	encodedAddr string,
	//	priKey crypto.PrivateKey,
	//	clock clock.Clock,
	//)
	require := require.New(t)
	cfg := config.Default.Consensus.RollDPoS
	rctx := newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, nil, nil, nil, nil, nil, "", nil, nil)
	require.NotNil(rctx)
}
