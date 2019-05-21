// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package e2etest

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"go.uber.org/zap"
	_ "go.uber.org/automaxprocs"

	"github.com/iotexproject/iotex-core/pkg/probe"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/server/itx"

)

func BenchmarkConsensus(b *testing.B) {

}
func TestRunOnce(t *testing.T) {
	runOnce()
}
func runOnce() {
	numNodes := 36
	configs := make([]config.Config, numNodes)

	for i := 0; i < numNodes; i++ {
		testTrieFile, _ := ioutil.TempFile(os.TempDir(), "trie")
		testTriePath := testTrieFile.Name()
		testDBFile, _ := ioutil.TempFile(os.TempDir(), "db")
		testDBPath := testDBFile.Name()
		fmt.Println(testTriePath)
		fmt.Println(testDBPath)
		networkPort := 4689 + i
		apiPort := 14014 + i
		config := makeConfig(testDBPath, testTriePath, identityset.PrivateKey(1).HexString(),networkPort, apiPort)
		if i == 0 {
			config.Network.BootstrapNodes = []string{}
			config.Network.MasterKey = "bootnode"
		}
		configs[i] = config
	}
	// Create mini-cluster
	svrs := make([]*itx.Server, numNodes)
	for i := 0; i < numNodes; i++ {
		if i != 0 {
			configs[i].Network.BootstrapNodes = []string{svrs[0].P2PAgent().Self()[0].String()}
		}
		svr, err := itx.NewServer(configs[i])
		if err != nil {
			log.L().Fatal("Failed to create server.", zap.Error(err))
		}
		svrs[i] = svr
	}
	// Create a probe server
	probeSvr := probe.New(7788)

	// Start mini-cluster
	for i := 0; i < numNodes; i++ {
		go itx.StartServer(context.Background(), svrs[i], probeSvr, configs[i])
	}
	for i := 0; i < numNodes; i++ {
		defer func() {
			testutil.WaitUntil(100*time.Millisecond, 20*time.Second, func() (b bool, e error) {
				return svrs[i].ChainService(1).Blockchain().TipHeight() >= 10, nil
			})
			svrs[i].Stop(context.Background())
		}()
	}
}
func makeConfig(
	chainDBPath,
	trieDBPath string,
	producerPriKey string,
	networkPort,
	apiPort int,
) config.Config {
	cfg := config.Default
	cfg.Network.Port = networkPort
	cfg.Chain.ChainDBPath = chainDBPath
	cfg.Chain.TrieDBPath = trieDBPath
	cfg.Chain.ProducerPrivKey = producerPriKey
	cfg.API.Port = apiPort

	cfg.Consensus.Scheme = config.RollDPoSScheme
	cfg.Consensus.RollDPoS.Delay = 6 * time.Second

	cfg.Genesis.BlockInterval = 10 * time.Second
	cfg.Genesis.Blockchain.NumDelegates = 24
	return cfg
}
