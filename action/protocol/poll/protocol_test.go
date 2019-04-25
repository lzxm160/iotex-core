// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package poll

import (
	"context"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/testaddress"
	"math/big"
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/iotexproject/iotex-election/test/mock/mock_committee"
	"github.com/iotexproject/iotex-election/types"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action/protocol/vote/candidatesutil"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/hash"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
)

func initConstruct(t *testing.T)(Protocol,context.Context,factory.WorkingSet,*types.ElectionResult){
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	cfg := config.Default
	ctx := protocol.WithRunActionsCtx(
		context.Background(),
		protocol.RunActionsCtx{
			BlockHeight: 123456,
		},
	)

	sf, err := factory.NewFactory(cfg, factory.InMemTrieOption())
	require.NoError(err)
	require.NoError(sf.Start(ctx))
	defer func() {
		require.NoError(sf.Stop(ctx))
	}()
	ws, err := sf.NewWorkingSet()
	require.NoError(err)
	committee := mock_committee.NewMockCommittee(ctrl)
	r := types.NewElectionResultForTest(time.Now())
	committee.EXPECT().ResultByHeight(uint64(123456)).Return(r, nil).AnyTimes()
	p, err := NewGovernanceChainCommitteeProtocol(
		nil,
		committee,
		uint64(123456),
		func(uint64) (time.Time, error) { return time.Now(), nil },
		func(uint64) uint64 { return 1 },
		func(uint64) uint64 { return 1 },
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
	)
	require.NoError(err)
	return p,ctx,ws,r
}
func TestInitialize(t *testing.T) {
	require := require.New(t)
	p,ctx,ws,r:=initConstruct(t)
	require.NoError(p.Initialize(ctx, ws))
	var sc state.CandidateList
	require.NoError(ws.State(candidatesutil.ConstructKey(1), &sc))
	candidates, err := state.CandidatesToMap(sc)
	require.NoError(err)
	require.Equal(2, len(candidates))
	for _, d := range r.Delegates() {
		operator := string(d.OperatorAddress())
		addr, err := address.FromString(operator)
		require.NoError(err)
		c, ok := candidates[hash.BytesToHash160(addr.Bytes())]
		require.True(ok)
		require.Equal(addr.String(), c.Address)
	}
}

func TestHandle(t *testing.T) {
	require := require.New(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	sm := mock_chainmanager.NewMockStateManager(ctrl)
	cb := db.NewCachedBatch()
	sm.EXPECT().GetCachedBatch().Return(cb).AnyTimes()
	sm.EXPECT().State(gomock.Any(), gomock.Any()).Return(state.ErrStateNotExist).AnyTimes()
	sm.EXPECT().PutState(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	sm.EXPECT().Snapshot().Return(1).AnyTimes()
	p,ctx,ws,_:=initConstruct(t)
	require.NoError(p.Initialize(ctx, ws))

	// wrong action
	recipientAddr := testaddress.Addrinfo["alfa"]
	senderKey := testaddress.Keyinfo["producer"]
	tsf, err := action.NewTransfer(0, big.NewInt(10), recipientAddr.String(), []byte{}, uint64(100000), big.NewInt(10))
	require.NoError(err)
	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(tsf).Build()
	selp, err := action.Sign(elp, senderKey.PriKey)
	require.NoError(err)
	require.NotNil(selp)
	// Case 1: wrong action type
	receipt,err:=p.Handle(ctx,selp.Action(),nil)
	require.Nil(err)
	require.Nil(receipt)
	// Case 2: right action type,setCandidates error
	var sc state.CandidateList
	require.NoError(ws.State(candidatesutil.ConstructKey(1), &sc))
	act := action.NewPutPollResult(1, 123456, sc)
	elp = bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act).Build()
	selp, err = action.Sign(elp, senderKey.PriKey)
	require.NoError(err)
	require.NotNil(selp)
	receipt,err=p.Handle(ctx,selp.Action(),sm)
	require.Error(err)
	require.Nil(receipt)
	// Case 3: all right
	p3,ctx3,ws3,_:=initConstruct(t)
	require.NoError(p3.Initialize(ctx3, ws3))
	var sc3 state.CandidateList
	require.NoError(ws3.State(candidatesutil.ConstructKey(1), &sc3))
	act3 := action.NewPutPollResult(1, 1, sc3)
	elp = bd.SetGasLimit(uint64(100000)).
		SetGasPrice(big.NewInt(10)).
		SetAction(act3).Build()
	selp3, err := action.Sign(elp, senderKey.PriKey)
	require.NoError(err)
	require.NotNil(selp3)
	receipt,err=p.Handle(ctx3,selp3.Action(),sm)
	require.NoError(err)
	require.NotNil(receipt)
}

func TestProtocol_Validate(t *testing.T) {
}
