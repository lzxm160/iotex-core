// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"io/ioutil"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/staking"
	"github.com/iotexproject/iotex-core/state/factory"

	"github.com/iotexproject/iotex-core/blockchain"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/p2p"
	"github.com/iotexproject/iotex-core/server/itx"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
)

type candidateRegisterCfg struct {
	Nonce           uint64
	Name            string
	OperatorAddrStr string
	RewardAddrStr   string
	OwnerAddrStr    string
	AmountStr       string
	Duration        uint32
	AutoStake       bool
	Payload         []byte
	GasLimit        uint64
	GasPrice        *big.Int
	signer          crypto.PrivateKey
	signerAddr      address.Address
	vote            string
}

var (
	candidateRegisterTests = []candidateRegisterCfg{
		{
			uint64(10), "test1", "io10a298zmzvrt4guq79a9f4x7qedj59y7ery84he", "io13sj9mzpewn25ymheukte4v39hvjdtrfp00mlyv", "io19d0p3ah4g8ww9d7kcxfq87yxe7fnr8rpth5shj", "100", uint32(10000), false, []byte("payload"), uint64(1000000), big.NewInt(1000),
			identityset.PrivateKey(27),
			identityset.Address(27),
			"128",
		},
		{
			uint64(10), "test2", "io14gnqxf9dpkn05g337rl7eyt2nxasphf5m6n0rd", "io1cl6rl2ev5dfa988qmgzg2x4hfazmp9vn2g66ng", "io1fxzh50pa6qc6x5cprgmgw4qrp5vw97zk5pxt3q", "1000", uint32(10000), true, []byte("payload"), uint64(1000000), big.NewInt(1000), identityset.PrivateKey(28),
			identityset.Address(28),
			"128",
		},
	}
)

func TestCandidateRegister(t *testing.T) {
	require := require.New(t)
	cfg, err := newCandidateRegisterCfg(t)
	require.NoError(err)
	// Create server
	ctx := context.Background()
	svr, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(svr.Start(ctx))

	chainID := cfg.Chain.ID
	bc := svr.ChainService(chainID).Blockchain()
	sf := svr.ChainService(chainID).StateFactory()
	dao := svr.ChainService(chainID).BlockDAO()
	require.NotNil(bc)
	require.NotNil(sf)
	require.NotNil(dao)
	require.NotNil(svr.P2PAgent())
	require.NoError(addCandidateRegister(bc))
	for _, test := range candidateRegisterTests {
		owner, err := address.FromString(test.OwnerAddrStr)
		require.NoError(err)
		c, err := getCandidate(sf, owner)
		require.NoError(err)
		require.Equal(test.Name, c.Name)
		require.Equal(test.OperatorAddrStr, c.Operator.String())
		require.Equal(test.RewardAddrStr, c.Reward.String())
		require.Equal(test.OwnerAddrStr, c.Owner.String())
		require.Equal(test.vote, c.Votes.String())
		require.Equal(test.AmountStr, c.SelfStake.String())
	}
	blk, err := dao.GetBlockByHeight(1)
	require.NoError(err)

	testDBFile2, _ := ioutil.TempFile(os.TempDir(), dBPath2)
	testDBPath2 := testDBFile2.Name()
	testTrieFile2, _ := ioutil.TempFile(os.TempDir(), triePath2)
	testTriePath2 := testTrieFile2.Name()
	indexDBFile2, _ := ioutil.TempFile(os.TempDir(), dBPath2)
	indexDBPath2 := indexDBFile2.Name()

	cfg, err = newTestConfig()
	require.NoError(err)
	cfg.Chain.TrieDBPath = testTriePath2
	cfg.Chain.ChainDBPath = testDBPath2
	cfg.Chain.IndexDBPath = indexDBPath2

	// Create client
	cfg.Network.BootstrapNodes = []string{validNetworkAddr(svr.P2PAgent().Self())}

	cfg.BlockSync.Interval = 1 * time.Second
	cli, err := itx.NewServer(cfg)
	require.NoError(err)
	require.NoError(cli.Start(ctx))
	require.NotNil(cli.ChainService(chainID).Blockchain())
	require.NotNil(cli.P2PAgent())

	defer func() {
		require.NoError(cli.Stop(ctx))
		require.NoError(svr.Stop(ctx))
	}()

	err = testutil.WaitUntil(time.Millisecond*100, time.Second*60, func() (bool, error) {
		peers, err := svr.P2PAgent().Neighbors(context.Background())
		return len(peers) >= 1, err
	})
	require.NoError(err)

	err = svr.P2PAgent().BroadcastOutbound(
		p2p.WitContext(ctx, p2p.Context{ChainID: cfg.Chain.ID}),
		blk.ConvertToBlockPb(),
	)
	require.NoError(err)
	check := testutil.CheckCondition(func() (bool, error) {
		blk1, err := cli.ChainService(chainID).BlockDAO().GetBlockByHeight(1)
		if err != nil {
			return false, nil
		}
		return blk.HashBlock() == blk1.HashBlock(), nil
	})
	require.NoError(testutil.WaitUntil(time.Millisecond*100, time.Second*60, check))

	// verify received candidate
	//for _, test := range candidateRegisterTests {
	//	c, err := getCandidate(sf, test.signerAddr)
	//	require.NoError(err)
	//	require.Equal(test.Name, c.Name)
	//	require.Equal(test.OperatorAddrStr, c.Operator.String())
	//	require.Equal(test.RewardAddrStr, c.Reward.String())
	//	require.Equal(test.OwnerAddrStr, c.Owner.String())
	//	require.Equal(test.AmountStr, c.Votes.String())
	//	require.Equal(test.AmountStr, c.SelfStake.String())
	//}

	t.Log("4 blocks received correctly")
}

func addCandidateRegister(bc blockchain.Blockchain) error {
	actionMap := make(map[string][]action.SealedEnvelope)

	for _, test := range candidateRegisterTests {
		cr, err := action.NewCandidateRegister(test.Nonce, test.Name, test.OperatorAddrStr, test.RewardAddrStr, test.OwnerAddrStr, test.AmountStr, test.Duration, test.AutoStake, test.Payload, test.GasLimit, test.GasPrice)
		if err != nil {
			return err
		}

		bd := &action.EnvelopeBuilder{}
		elp := bd.SetGasLimit(test.GasLimit).
			SetGasPrice(test.GasPrice).
			SetAction(cr).Build()

		selp, err := action.Sign(elp, test.signer)
		actionMap[test.signerAddr.String()] = append(actionMap[test.signerAddr.String()], selp)
	}

	blk, err := bc.MintNewBlock(
		actionMap,
		testutil.TimestampNow(),
	)
	if err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	return nil
}

func getCandidate(sr protocol.StateReader, name address.Address) (*staking.Candidate, error) {
	key := make([]byte, len(name.Bytes()))
	copy(key, name.Bytes())

	var d staking.Candidate
	_, err := sr.State(&d, protocol.NamespaceOption(factory.CandidateNameSpace), protocol.KeyOption(key))
	return &d, err
}

func newCandidateRegisterCfg(t *testing.T) (config.Config, error) {
	require := require.New(t)
	cfg, err := newTestConfig()
	require.NoError(err)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()
	testDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	testDBPath := testDBFile.Name()
	indexDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	indexDBPath := indexDBFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath
	cfg.Chain.IndexDBPath = indexDBPath
	for _, test := range candidateRegisterTests {
		cfg.Genesis.InitBalanceMap[test.signerAddr.String()] = "1000000000"
	}
	return cfg, nil
}
