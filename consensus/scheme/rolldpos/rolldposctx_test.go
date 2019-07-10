// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/test/identityset"

	"github.com/facebookgo/clock"

	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"

	"github.com/iotexproject/iotex-core/config"
	"github.com/stretchr/testify/require"
)

func TestRollDPoSCtx(t *testing.T) {
	require := require.New(t)
	cfg := config.Default.Consensus.RollDPoS

	// case 1:panic because of chain is nil
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, nil, nil, nil, nil, nil, "", nil, nil)
	}, "chain is nil")

	// case 2:panic because of rp is nil
	b, _ := makeChain(t)
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, b, nil, nil, nil, nil, "", nil, nil)
	}, "rp is nil")

	// case 3:panic because of clock is nil
	rp := rolldpos.NewProtocol(
		config.Default.Genesis.NumCandidateDelegates,
		config.Default.Genesis.NumDelegates,
		config.Default.Genesis.NumSubEpochs,
	)
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, b, nil, rp, nil, nil, "", nil, nil)
	}, "clock is nil")

	// case 4:panic because of fsm time bigger than block interval
	c := clock.New()
	cfg.FSM.AcceptBlockTTL = time.Second * 10
	cfg.FSM.AcceptProposalEndorsementTTL = time.Second
	cfg.FSM.AcceptLockEndorsementTTL = time.Second
	cfg.FSM.CommitTTL = time.Second
	require.Panics(func() {
		newRollDPoSCtx(cfg, true, time.Second*10, time.Second, true, b, nil, rp, nil, nil, "", nil, c)
	}, "fsm's time is bigger than block interval")

	// case 5:normal
	rctx := newRollDPoSCtx(cfg, true, time.Second*20, time.Second, true, b, nil, rp, nil, nil, "", nil, c)
	require.NotNil(rctx)
}

func TestCheckVoteEndorser(t *testing.T) {
	require := require.New(t)
	cfg := config.Default.Consensus.RollDPoS
	b, _ := makeChain(t)
	rp := rolldpos.NewProtocol(
		config.Default.Genesis.NumCandidateDelegates,
		config.Default.Genesis.NumDelegates,
		config.Default.Genesis.NumSubEpochs,
	)
	c := clock.New()
	rctx := newRollDPoSCtx(cfg, true, time.Second*20, time.Second, true, b, nil, rp, nil, nil, "", nil, c)
	require.NotNil(rctx)

	// case 1:endorser nil caused panic
	require.Panics(func() { rctx.CheckBlockProposer(0, nil, nil) }, "")

	//case 2:endorser address error
	en := &endorsement.Endorsement{time.Now(), identityset.PrivateKey(0).PublicKey(), nil}
	require.Error(rctx.CheckBlockProposer(0, nil, en))
}
