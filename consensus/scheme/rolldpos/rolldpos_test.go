// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/iotexproject/iotex-core/action/protocol/account/util"

	"github.com/facebookgo/clock"
	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/context"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/poll"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/blockchain/blockdao"
	"github.com/iotexproject/iotex-core/blockchain/genesis"
	"github.com/iotexproject/iotex-core/blockindex"
	"github.com/iotexproject/iotex-core/config"
	cp "github.com/iotexproject/iotex-core/crypto"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/endorsement"
	"github.com/iotexproject/iotex-core/p2p/node"
	"github.com/iotexproject/iotex-core/state"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/test/mock/mock_actpool"
	"github.com/iotexproject/iotex-core/test/mock/mock_blockchain"
	"github.com/iotexproject/iotex-core/testutil"
)

type addrKeyPair struct {
	priKey      crypto.PrivateKey
	encodedAddr string
}

func TestNewRollDPoS(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	cfg := config.Default
	rp := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	t.Run("normal", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		r, err := NewRollDPoSBuilder().
			SetConfig(cfg).
			SetAddr(identityset.Address(0).String()).
			SetPriKey(sk).
			SetChainManager(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			RegisterProtocol(rp).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
	t.Run("mock-clock", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		r, err := NewRollDPoSBuilder().
			SetConfig(cfg).
			SetAddr(identityset.Address(0).String()).
			SetPriKey(sk).
			SetChainManager(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetClock(clock.NewMock()).
			RegisterProtocol(rp).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
		_, ok := r.ctx.clock.(*clock.Mock)
		assert.True(t, ok)
	})

	t.Run("root chain API", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		r, err := NewRollDPoSBuilder().
			SetConfig(cfg).
			SetAddr(identityset.Address(0).String()).
			SetPriKey(sk).
			SetChainManager(mock_blockchain.NewMockBlockchain(ctrl)).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			SetClock(clock.NewMock()).
			RegisterProtocol(rp).
			Build()
		assert.NoError(t, err)
		assert.NotNil(t, r)
	})
	t.Run("missing-dep", func(t *testing.T) {
		sk := identityset.PrivateKey(0)
		r, err := NewRollDPoSBuilder().
			SetConfig(cfg).
			SetAddr(identityset.Address(0).String()).
			SetPriKey(sk).
			SetActPool(mock_actpool.NewMockActPool(ctrl)).
			SetBroadcast(func(_ proto.Message) error {
				return nil
			}).
			RegisterProtocol(rp).
			Build()
		assert.Error(t, err)
		assert.Nil(t, r)
	})
}
func makeBlock(t *testing.T, accountIndex, numOfEndosements int, makeInvalidEndorse bool, height int) *block.Block {
	unixTime := 1500000000
	blkTime := int64(-1)
	if height != 9 {
		height = 9
		blkTime = int64(-7723372030)
	}
	timeT := time.Unix(blkTime, 0)
	rap := block.RunnableActionsBuilder{}
	ra := rap.
		SetHeight(uint64(height)).
		SetTimeStamp(timeT).
		Build(identityset.PrivateKey(accountIndex).PublicKey())
	blk, err := block.NewBuilder(ra).
		SetVersion(1).
		SetReceiptRoot(hash.Hash256b([]byte("hello, world!"))).
		SetDeltaStateDigest(hash.Hash256b([]byte("world, hello!"))).
		SetPrevBlockHash(hash.Hash256b([]byte("hello, block!"))).
		SignAndBuild(identityset.PrivateKey(accountIndex))
	require.NoError(t, err)
	footerForBlk := &block.Footer{}
	typesFooter := iotextypes.BlockFooter{}

	for i := 0; i < numOfEndosements; i++ {
		timeTime := time.Unix(int64(unixTime), 0)
		hs := blk.HashBlock()
		var consensusVote *ConsensusVote
		if makeInvalidEndorse {
			consensusVote = NewConsensusVote(hs[:], LOCK)
		} else {
			consensusVote = NewConsensusVote(hs[:], COMMIT)
		}
		en, err := endorsement.Endorse(identityset.PrivateKey(i), consensusVote, timeTime)
		require.NoError(t, err)
		enProto, err := en.Proto()
		require.NoError(t, err)
		typesFooter.Endorsements = append(typesFooter.Endorsements, enProto)
	}
	ts, err := ptypes.TimestampProto(time.Unix(int64(unixTime), 0))
	require.NoError(t, err)
	typesFooter.Timestamp = ts
	require.NotNil(t, typesFooter.Timestamp)
	err = footerForBlk.ConvertFromBlockFooterPb(&typesFooter)
	require.NoError(t, err)
	blk.Footer = *footerForBlk
	return &blk
}

func TestValidateBlockFooter(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	candidates := make([]string, 5)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = identityset.Address(i).String()
	}
	clock := clock.NewMock()
	blockHeight := uint64(8)
	footer := &block.Footer{}
	blockchain := mock_blockchain.NewMockBlockchain(ctrl)
	blockchain.EXPECT().BlockFooterByHeight(blockHeight).Return(footer, nil).Times(5)
	blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{Address: candidates[0]},
		{Address: candidates[1]},
		{Address: candidates[2]},
		{Address: candidates[3]},
		{Address: candidates[4]},
	}, nil).AnyTimes()

	sk1 := identityset.PrivateKey(1)
	cfg := config.Default
	cfg.Genesis.NumDelegates = 4
	cfg.Genesis.NumSubEpochs = 1
	cfg.Genesis.BlockInterval = 10 * time.Second
	cfg.Genesis.Timestamp = int64(1500000000)
	blockchain.EXPECT().Genesis().Return(cfg.Genesis).Times(5)
	rp := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	r, err := NewRollDPoSBuilder().
		SetConfig(cfg).
		SetAddr(identityset.Address(1).String()).
		SetPriKey(sk1).
		SetChainManager(blockchain).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetBroadcast(func(_ proto.Message) error {
			return nil
		}).
		SetClock(clock).
		RegisterProtocol(rp).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)

	// all right
	blk := makeBlock(t, 1, 4, false, 9)
	err = r.ValidateBlockFooter(blk)
	require.NoError(t, err)

	// Proposer is wrong
	blk = makeBlock(t, 0, 4, false, 9)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)

	// Not enough endorsements
	blk = makeBlock(t, 1, 2, false, 9)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)

	// round information is wrong
	blk = makeBlock(t, 1, 4, false, 0)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)

	// Some endorsement is invalid
	blk = makeBlock(t, 1, 4, true, 9)
	err = r.ValidateBlockFooter(blk)
	require.Error(t, err)
}

func TestRollDPoS_Metrics(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	candidates := make([]string, 5)
	for i := 0; i < len(candidates); i++ {
		candidates[i] = identityset.Address(i).String()
	}

	clock := clock.NewMock()
	blockHeight := uint64(8)
	footer := &block.Footer{}
	blockchain := mock_blockchain.NewMockBlockchain(ctrl)
	blockchain.EXPECT().TipHeight().Return(blockHeight).Times(1)
	blockchain.EXPECT().BlockFooterByHeight(blockHeight).Return(footer, nil).Times(2)
	blockchain.EXPECT().CandidatesByHeight(gomock.Any()).Return([]*state.Candidate{
		{Address: candidates[0]},
		{Address: candidates[1]},
		{Address: candidates[2]},
		{Address: candidates[3]},
		{Address: candidates[4]},
	}, nil).AnyTimes()

	sk1 := identityset.PrivateKey(1)
	cfg := config.Default
	cfg.Genesis.NumDelegates = 4
	cfg.Genesis.NumSubEpochs = 1
	cfg.Genesis.BlockInterval = 10 * time.Second
	cfg.Genesis.Timestamp = int64(1500000000)
	blockchain.EXPECT().Genesis().Return(cfg.Genesis).Times(2)
	rp := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	r, err := NewRollDPoSBuilder().
		SetConfig(cfg).
		SetAddr(identityset.Address(1).String()).
		SetPriKey(sk1).
		SetChainManager(blockchain).
		SetActPool(mock_actpool.NewMockActPool(ctrl)).
		SetBroadcast(func(_ proto.Message) error {
			return nil
		}).
		SetClock(clock).
		RegisterProtocol(rp).
		Build()
	require.NoError(t, err)
	require.NotNil(t, r)
	clock.Add(r.ctx.BlockInterval(blockHeight))
	require.NoError(t, r.ctx.Start(context.Background()))
	r.ctx.round, err = r.ctx.RoundCalc().UpdateRound(r.ctx.round, blockHeight+1, r.ctx.BlockInterval(blockHeight+1), clock.Now(), 2*time.Second)
	require.NoError(t, err)

	m, err := r.Metrics()
	require.NoError(t, err)
	assert.Equal(t, uint64(3), m.LatestEpoch)

	cp.SortCandidates(candidates, rp.GetEpochHeight(m.LatestEpoch), cp.CryptoSeed)
	assert.Equal(t, candidates[:4], m.LatestDelegates)
	assert.Equal(t, candidates[1], m.LatestBlockProducer)
}

// E2E RollDPoS tests bellow

type directOverlay struct {
	addr  net.Addr
	peers map[net.Addr]*RollDPoS
}

func (o *directOverlay) Start(_ context.Context) error { return nil }

func (o *directOverlay) Stop(_ context.Context) error { return nil }

func (o *directOverlay) Broadcast(msg proto.Message) error {
	// Only broadcast consensus message
	if cMsg, ok := msg.(*iotextypes.ConsensusMessage); ok {
		for _, r := range o.peers {
			if err := r.HandleConsensusMsg(cMsg); err != nil {
				return errors.Wrap(err, "error when handling consensus message directly")
			}
		}
	}
	return nil
}

func (o *directOverlay) Tell(uint32, net.Addr, proto.Message) error { return nil }

func (o *directOverlay) Self() net.Addr { return o.addr }

func (o *directOverlay) GetPeers() []net.Addr {
	addrs := make([]net.Addr, 0, len(o.peers))
	for addr := range o.peers {
		addrs = append(addrs, addr)
	}
	return addrs
}

func TestRollDPoSConsensus(t *testing.T) {
	newConsensusComponents := func(numNodes int) ([]*RollDPoS, []*directOverlay, []blockchain.Blockchain) {
		cfg := newConfig(numNodes)
		chainAddrs := make([]*addrKeyPair, 0, numNodes)
		networkAddrs := make([]net.Addr, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			sk := identityset.PrivateKey(i)
			addr := addrKeyPair{
				encodedAddr: identityset.Address(i).String(),
				priKey:      sk,
			}
			chainAddrs = append(chainAddrs, &addr)
			networkAddrs = append(networkAddrs, node.NewTCPNode(fmt.Sprintf("127.0.0.%d:4689", i+1)))
		}

		chainRawAddrs := make([]string, 0, numNodes)
		addressMap := make(map[string]*addrKeyPair)
		for _, addr := range chainAddrs {
			chainRawAddrs = append(chainRawAddrs, addr.encodedAddr)
			addressMap[addr.encodedAddr] = addr
		}
		cp.SortCandidates(chainRawAddrs, 1, cp.CryptoSeed)
		for i, rawAddress := range chainRawAddrs {
			chainAddrs[i] = addressMap[rawAddress]
		}

		candidatesByHeightFunc := func(_ uint64) ([]*state.Candidate, error) {
			candidates := make([]*state.Candidate, 0, numNodes)
			for _, addr := range chainAddrs {
				candidates = append(candidates, &state.Candidate{Address: addr.encodedAddr})
			}
			return candidates, nil
		}

		chains := make([]blockchain.Blockchain, 0, numNodes)
		p2ps := make([]*directOverlay, 0, numNodes)
		cs := make([]*RollDPoS, 0, numNodes)
		for i := 0; i < numNodes; i++ {
			ctx := context.Background()
			cfg[i].Chain.ProducerPrivKey = hex.EncodeToString(chainAddrs[i].priKey.Bytes())
			sf, err := factory.NewFactory(cfg[i], factory.DefaultTrieOption())
			require.NoError(t, err)
			// new sf for blockchain to start
			sf2, err := factory.NewFactory(cfg[i], factory.DefaultTrieOption())
			require.NoError(t, err)
			require.NoError(t, sf.Start(ctx))
			for j := 0; j < numNodes; j++ {

				ws, err := sf.NewWorkingSet(nil)
				require.NoError(t, err)
				_, err = accountutil.LoadOrCreateAccount(ws, chainRawAddrs[j], big.NewInt(0))
				require.NoError(t, err)
				gasLimit := testutil.TestGasLimit
				wsctx := protocol.WithRunActionsCtx(ctx,
					protocol.RunActionsCtx{
						Producer: identityset.Address(27),
						GasLimit: gasLimit,
						Genesis:  cfg[i].Genesis,
					})
				_, err = ws.RunActions(wsctx, 0, nil)
				require.NoError(t, err)
				require.NoError(t, sf.Commit(ws))

			}
			require.NoError(t, sf.Stop(ctx))
			registry := protocol.Registry{}
			acc := account.NewProtocol()
			require.NoError(t, registry.Register(account.ProtocolID, acc))
			rp := rolldpos.NewProtocol(cfg[i].Genesis.NumCandidateDelegates, cfg[i].Genesis.NumDelegates, cfg[i].Genesis.NumSubEpochs)
			require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))

			dbConfig := cfg[i].DB
			dbConfig.DbPath = cfg[i].Chain.IndexDBPath
			indexer, err := blockindex.NewIndexer(db.NewBoltDB(dbConfig), cfg[i].Genesis.Hash())
			require.NoError(t, err)
			dbConfig.DbPath = cfg[i].Chain.ChainDBPath
			dao := blockdao.NewBlockDAO(db.NewBoltDB(dbConfig), indexer, cfg[i].Chain.CompressBlock, dbConfig)

			chain := blockchain.NewBlockchain(
				cfg[i],
				dao,
				blockchain.PrecreatedStateFactoryOption(sf2),
				blockchain.RegistryOption(&registry),
			)
			evm := execution.NewProtocol(chain.BlockDAO().GetBlockHash)
			require.NoError(t, registry.Register(execution.ProtocolID, evm))
			p := poll.NewLifeLongDelegatesProtocol(cfg[i].Genesis.Delegates)
			rolldposProtocol := rolldpos.NewProtocol(
				genesis.Default.NumCandidateDelegates,
				genesis.Default.NumDelegates,
				genesis.Default.NumSubEpochs,
				rolldpos.EnableDardanellesSubEpoch(cfg[i].Genesis.DardanellesBlockHeight, cfg[i].Genesis.DardanellesNumSubEpochs),
			)
			r := rewarding.NewProtocol(chain, rolldposProtocol)
			require.NoError(t, registry.Register(rewarding.ProtocolID, r))
			require.NoError(t, registry.Register(poll.ProtocolID, p))

			chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain.Factory().Nonce))
			chain.Validator().AddActionValidators(acc, evm, r)

			chains = append(chains, chain)

			actPool, err := actpool.NewActPool(chain, cfg[i].ActPool, actpool.EnableExperimentalActions())
			require.NoError(t, err)

			p2p := &directOverlay{
				addr:  networkAddrs[i],
				peers: make(map[net.Addr]*RollDPoS),
			}
			p2ps = append(p2ps, p2p)

			consensus, err := NewRollDPoSBuilder().
				SetAddr(chainAddrs[i].encodedAddr).
				SetPriKey(chainAddrs[i].priKey).
				SetConfig(cfg[i]).
				SetChainManager(chain).
				SetActPool(actPool).
				SetBroadcast(p2p.Broadcast).
				SetCandidatesByHeightFunc(candidatesByHeightFunc).
				RegisterProtocol(rp).
				Build()
			require.NoError(t, err)

			cs = append(cs, consensus)
		}
		for i := 0; i < numNodes; i++ {
			for j := 0; j < numNodes; j++ {
				if i != j {
					p2ps[i].peers[p2ps[j].addr] = cs[j]
				}
			}
		}
		return cs, p2ps, chains
	}

	t.Run("1-block", func(t *testing.T) {
		// TODO: fix and enable the test
		t.Skip()

		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 10*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if chain.TipHeight() < 1 {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("1-epoch", func(t *testing.T) {
		if testing.Short() {
			t.Skip("Skip the 1-epoch test in short mode.")
		}
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 100*time.Second, func() (bool, error) {
			for _, chain := range chains {
				if chain.TipHeight() < 48 {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("network-partition-time-rotation", func(t *testing.T) {
		// TODO: fix and enable the test
		t.Skip()

		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				cs[idx].ctx.roundCalc.timeBasedRotation = true
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()

		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 60*time.Second, func() (bool, error) {
			for i, chain := range chains {
				if i == 1 {
					continue
				}
				if chain.TipHeight() < 4 {
					return false, nil
				}
			}
			return true, nil
		}))
	})

	t.Run("proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 1 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[1].addr)
			}
		}

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		time.Sleep(5 * time.Second)
		for _, chain := range chains {
			header, err := chain.BlockHeaderByHeight(1)
			assert.Nil(t, header)
			assert.Error(t, err)
		}
	})

	t.Run("non-proposer-network-partition-blocking", func(t *testing.T) {
		ctx := context.Background()
		cs, p2ps, chains := newConsensusComponents(24)
		// 1 should be the block 1's proposer
		for i, p2p := range p2ps {
			if i == 0 {
				p2p.peers = make(map[net.Addr]*RollDPoS)
			} else {
				delete(p2p.peers, p2ps[0].addr)
			}
		}

		for i := 0; i < 24; i++ {
			require.NoError(t, chains[i].Start(ctx))
			require.NoError(t, p2ps[i].Start(ctx))
		}
		wg := sync.WaitGroup{}
		wg.Add(24)
		for i := 0; i < 24; i++ {
			go func(idx int) {
				defer wg.Done()
				err := cs[idx].Start(ctx)
				require.NoError(t, err)
			}(i)
		}
		wg.Wait()

		defer func() {
			for i := 0; i < 24; i++ {
				require.NoError(t, cs[i].Stop(ctx))
				require.NoError(t, p2ps[i].Stop(ctx))
				require.NoError(t, chains[i].Stop(ctx))
			}
		}()
		assert.NoError(t, testutil.WaitUntil(200*time.Millisecond, 60*time.Second, func() (bool, error) {
			for i, chain := range chains {
				if i == 0 {
					continue
				}
				if chain.TipHeight() < 2 {
					return false, nil
				}
			}
			return true, nil
		}))
		for i, chain := range chains {
			header, err := chain.BlockHeaderByHeight(1)
			if i == 0 {
				assert.Nil(t, header)
				assert.Error(t, err)
			} else {
				assert.NotNil(t, header)
				assert.NoError(t, err)
			}
		}
	})
}

func newConfig(numNodes int) (ret []config.Config) {
	for i := 0; i < numNodes; i++ {
		cfg := config.Default
		cfg.Consensus.RollDPoS.Delay = 300 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.AcceptBlockTTL = 800 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.AcceptProposalEndorsementTTL = 400 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.AcceptLockEndorsementTTL = 400 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.CommitTTL = 400 * time.Millisecond
		cfg.Consensus.RollDPoS.FSM.UnmatchedEventTTL = time.Second
		cfg.Consensus.RollDPoS.FSM.UnmatchedEventInterval = 10 * time.Millisecond
		cfg.Consensus.RollDPoS.ToleratedOvertime = 200 * time.Millisecond
		testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
		testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
		testIndexFile, _ := ioutil.TempFile(os.TempDir(), "index")
		testConsensusFile, _ := ioutil.TempFile(os.TempDir(), "consensus")
		gravityChainDBFile, _ := ioutil.TempFile(os.TempDir(), "poll")
		cfg.Chain.TrieDBPath = testTrieFile.Name()
		cfg.Chain.ChainDBPath = testDBFile.Name()
		cfg.Chain.IndexDBPath = testIndexFile.Name()
		cfg.Chain.GravityChainDB.DbPath = gravityChainDBFile.Name()
		cfg.Consensus.RollDPoS.ConsensusDBPath = testConsensusFile.Name()
		cfg.Genesis.BlockInterval = 2 * time.Second
		cfg.Genesis.Blockchain.NumDelegates = uint64(numNodes)
		cfg.Genesis.Blockchain.NumSubEpochs = 1
		cfg.Genesis.EnableGravityChainVoting = true
		cfg.Plugins[config.GatewayPlugin] = true
		cfg.Chain.EnableAsyncIndexWrite = false
		cfg.ActPool.MinGasPriceStr = "0"
		cfg.API.RangeQueryLimit = 100

		ret = append(ret, cfg)
	}

	return
}
