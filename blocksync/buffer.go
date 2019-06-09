// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blocksync

import (
	"sync"
	"time"

	"github.com/iotexproject/iotex-election/committee"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/actpool"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/consensus"
	"github.com/iotexproject/iotex-core/pkg/log"
)

type bCheckinResult int

const (
	bCheckinValid bCheckinResult = iota + 1
	bCheckinLower
	bCheckinExisting
	bCheckinHigher
	bCheckinSkipNil
)

// blockBuffer is used to keep in-coming block in order.
type blockBuffer struct {
	mu                sync.RWMutex
	blocks            map[uint64]*block.Block
	bc                blockchain.Blockchain
	ap                actpool.ActPool
	cs                consensus.Consensus
	bufferSize        uint64
	intervalSize      uint64
	commitHeight      uint64 // last commit block height
	electionCommittee committee.Committee
	numDelegates      uint64
	numSubEpochs      uint64
}

// CommitHeight return the last commit block height
func (b *blockBuffer) CommitHeight() uint64 {
	return b.commitHeight
}

// Flush tries to put given block into buffer and flush buffer into blockchain.
func (b *blockBuffer) Flush(blk *block.Block) (bool, bCheckinResult) {
	b.mu.Lock()
	defer b.mu.Unlock()
	epochNum := (blk.Height()-1)/b.numDelegates/b.numSubEpochs + 1
	epochStartHeight := (epochNum-1)*b.numDelegates*b.numSubEpochs + 1
	localDbHeight := b.electionCommittee.LatestHeight()
	interval := epochStartHeight - blk.Height()
	requestHeight, err := b.electionCommittee.HeightByTime(blk.Header.Timestamp().Add(time.Second * 10 * time.Duration(interval)))
	if err != nil {
		return false, bCheckinValid
	}
	log.L().Error("",
		zap.Uint64("epochNum", epochNum),
		zap.Uint64("epochStartHeight", epochStartHeight),

		zap.Uint64("requesthei", requestHeight),
		zap.Uint64("localDbHeight", localDbHeight),
	)
	if requestHeight > localDbHeight {
		return false, bCheckinValid
	}

	if blk == nil {
		return false, bCheckinSkipNil
	}
	confirmedHeight := b.bc.TipHeight()
	// check
	blkHeight := blk.Height()
	if blkHeight <= confirmedHeight {
		return false, bCheckinLower
	}
	if _, ok := b.blocks[blkHeight]; ok {
		return false, bCheckinExisting
	}
	if blkHeight > confirmedHeight+b.bufferSize {
		return false, bCheckinHigher
	}
	b.blocks[blkHeight] = blk
	l := log.L().With(
		zap.Uint64("recvHeight", blkHeight),
		zap.Uint64("confirmedHeight", confirmedHeight),
		zap.String("source", "blockBuffer"))
	var heightToSync uint64
	for heightToSync = confirmedHeight + 1; heightToSync <= confirmedHeight+b.bufferSize; heightToSync++ {
		blk, ok := b.blocks[heightToSync]
		if !ok {
			break
		}
		delete(b.blocks, heightToSync)
		if err := commitBlock(b.bc, b.ap, b.cs, blk); err != nil && errors.Cause(err) != blockchain.ErrInvalidTipHeight {
			l.Error("Failed to commit the block.", zap.Error(err), zap.Uint64("syncHeight", heightToSync))
			break
		}
		b.commitHeight = heightToSync
		l.Info("Successfully committed block.", zap.Uint64("syncedHeight", heightToSync))
	}

	// clean up on memory leak
	if len(b.blocks) > int(b.bufferSize)*2 {
		l.Warn("blockBuffer is leaking memory.", zap.Int("bufferSize", len(b.blocks)))
		for h := range b.blocks {
			if h <= confirmedHeight {
				delete(b.blocks, h)
			}
		}
	}

	return heightToSync > blkHeight, bCheckinValid
}

// GetBlocksIntervalsToSync returns groups of syncBlocksInterval are missing upto targetHeight.
func (b *blockBuffer) GetBlocksIntervalsToSync(targetHeight uint64) []syncBlocksInterval {
	var (
		start    uint64
		startSet bool
		bi       []syncBlocksInterval
	)

	b.mu.RLock()
	defer b.mu.RUnlock()

	confirmedHeight := b.bc.TipHeight()
	// The sync range shouldn't go beyond tip height + buffer size to avoid being too aggressive
	if targetHeight > confirmedHeight+b.bufferSize {
		targetHeight = confirmedHeight + b.bufferSize
	}
	// The sync range should at least contain one interval to speculatively fetch missing blocks
	if targetHeight < confirmedHeight+b.intervalSize {
		targetHeight = confirmedHeight + b.intervalSize
	}

	var iLen uint64
	for h := confirmedHeight + 1; h <= targetHeight; h++ {
		if _, ok := b.blocks[h]; !ok {
			iLen++
			if !startSet {
				start = h
				startSet = true
			}
			if iLen >= b.intervalSize {
				bi = append(bi, syncBlocksInterval{Start: start, End: h})
				startSet = false
				iLen = 0
			}
			continue
		}
		if startSet {
			bi = append(bi, syncBlocksInterval{Start: start, End: h - 1})
			startSet = false
			iLen = 0
		}
	}

	// handle last interval
	if startSet {
		bi = append(bi, syncBlocksInterval{Start: start, End: targetHeight})
	}
	return bi
}

// bufSize return the bufferSize of buffer
func (b *blockBuffer) bufSize() uint64 {
	return b.bufferSize
}
