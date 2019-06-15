// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"errors"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/consensus"
	"go.uber.org/zap"
)

func commitBlock(bc blockchain.Blockchain, ap actpool.ActPool, cs consensus.Consensus, blk *block.Block) error {
	zap.L().Info("///////////////commitBlock", zap.Uint64("height", blk.Height()), zap.Error(errors.New("for call stack")))
	if err := cs.ValidateBlockFooter(blk); err != nil {
		return err
	}
	if err := bc.ValidateBlock(blk); err != nil {
		return err
	}
	if err := bc.CommitBlock(blk); err != nil {
		return err
	}
	cs.Calibrate(blk.Height())
	// remove transfers in this block from ActPool and reset ActPool state
	ap.Reset()
	return nil
}
