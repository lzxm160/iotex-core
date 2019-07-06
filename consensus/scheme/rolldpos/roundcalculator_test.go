// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package rolldpos

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/rewarding"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestRoundCalculator(t *testing.T) {
	require := require.New(t)
	//chain                  blockchain.Blockchain
	//blockInterval          time.Duration
	//toleratedOvertime      time.Duration
	//timeBasedRotation      bool
	//rp                     *rolldpos.Protocol
	//candidatesByHeightFunc CandidatesByHeightFunc
	rc := &roundCalculator{nil, time.Second, time.Second, true, nil, nil}
	require.NotNil(rc)
	require.Equal(time.Second, rc.BlockInterval())
	bc, roll := makeChain(t)
	rc = &roundCalculator{bc, time.Second, time.Second, true, roll, bc.CandidatesByHeight}

	// error for lastBlockTime.Before(now)
	_, _, err := rc.RoundInfo(1, time.Unix(1562382300, 0))
	require.Error(err)

	// height is 1
	roundNum, roundStartTime, err := rc.RoundInfo(1, time.Unix(1562382392, 0))
	require.NoError(err)
	fmt.Println(roundNum, ":", roundStartTime)
	require.Equal(2, roundNum)
	require.True(roundStartTime.Before(time.Unix(1562382392, 0)))
}
func makeChain(t *testing.T) (blockchain.Blockchain, *rolldpos.Protocol) {
	require := require.New(t)
	dBPath := "db.test"
	triePath := "trie.test"
	cfg := config.Default
	cfg.ActPool.MinGasPriceStr = "0"
	cfg.Consensus.Scheme = config.NOOPScheme
	cfg.Network.Port = testutil.RandomPort()
	cfg.API.Port = testutil.RandomPort()
	cfg.System.EnableExperimentalActions = true
	cfg.Genesis.Timestamp = 1562382372
	sk, err := crypto.GenerateKey()
	cfg.Chain.ProducerPrivKey = sk.HexString()
	require.Nil(err)
	testTrieFile, _ := ioutil.TempFile(os.TempDir(), triePath)
	testTriePath := testTrieFile.Name()

	testDBFile, _ := ioutil.TempFile(os.TempDir(), dBPath)
	testDBPath := testDBFile.Name()
	cfg.Chain.TrieDBPath = testTriePath
	cfg.Chain.ChainDBPath = testDBPath

	registry := protocol.Registry{}
	chain := blockchain.NewBlockchain(
		cfg,
		blockchain.DefaultStateFactoryOption(),
		blockchain.BoltDBDaoOption(),
		blockchain.RegistryOption(&registry),
	)
	rolldposProtocol := rolldpos.NewProtocol(
		cfg.Genesis.NumCandidateDelegates,
		cfg.Genesis.NumDelegates,
		cfg.Genesis.NumSubEpochs,
	)
	require.NoError(registry.Register(rolldpos.ProtocolID, rolldposProtocol))
	rewardingProtocol := rewarding.NewProtocol(chain, rolldposProtocol)
	registry.Register(rewarding.ProtocolID, rewardingProtocol)
	acc := account.NewProtocol(0)
	registry.Register(account.ProtocolID, acc)
	chain.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(chain))
	chain.Validator().AddActionValidators(acc, rewardingProtocol)
	chain.GetFactory().AddActionHandlers(acc, rewardingProtocol)
	ctx := context.Background()
	require.NoError(chain.Start(ctx))
	for i := 0; i < 5; i++ {
		blk, err := chain.MintNewBlock(
			nil,
			testutil.TimestampNow(),
		)
		require.NoError(err)
		require.NoError(chain.CommitBlock(blk))
	}
	require.Equal(uint64(5), chain.TipHeight())
	require.NoError(err)
	defer func() {
		require.NoError(chain.Stop(ctx))
	}()
	return chain, rolldposProtocol
}
