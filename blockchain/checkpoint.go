// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	blockNS       = "blockNS"
	blockReceipNS = "blockReceipNS"
	blockTopNS    = "blockTopNS"
	topBlockKey   = []byte("tbk")
)

type PutBlockToTrieDB struct {
	bc Blockchain
}

func NewPutBlockToTrieDB(bc Blockchain) *PutBlockToTrieDB {
	p := &PutBlockToTrieDB{
		bc,
	}
	return p
}
func (pb *PutBlockToTrieDB) writeBlock(blk *block.Block) error {
	ws, err := pb.bc.Factory().NewWorkingSet()
	if err != nil {
		return err
	}
	kv := ws.GetDB()
	blkHeight := blk.Height()
	blkSer, err := blk.Serialize()
	batch := db.NewBatch()
	heightKey := byteutil.Uint64ToBytes(blk.Height())
	batch.Put(blockNS, heightKey, blkSer, "failed to put block")
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			batch.Put(blockReceipNS, heightKey, receiptsBytes, "failed to put receipts")
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}
	return kv.Commit(batch)
}
func (pb *PutBlockToTrieDB) delBlock(height uint64) error {
	ws, err := pb.bc.Factory().NewWorkingSet()
	if err != nil {
		return err
	}
	kv := ws.GetDB()
	heightValue := byteutil.Uint64ToBytes(height)
	batch := db.NewBatch()
	batch.Delete(blockNS, heightValue, "failed to del block")
	batch.Delete(blockReceipNS, heightValue, "failed to del receipts")
	return kv.Commit(batch)
}
func (pb *PutBlockToTrieDB) writeBlockAndTop(blk *block.Block) error {
	err := pb.writeBlock(blk)
	if err != nil {
		return err
	}
	ws, err := pb.bc.Factory().NewWorkingSet()
	if err != nil {
		return err
	}
	kv := ws.GetDB()
	heightValue := byteutil.Uint64ToBytes(blk.Height())
	batch := db.NewBatch()
	batch.Put(blockTopNS, topBlockKey, heightValue, "failed to put block")
	return kv.Commit(batch)
}
func (pb *PutBlockToTrieDB) HandleBlock(blk *block.Block) error {
	err := pb.writeBlockAndTop(blk)
	if err != nil {
		return err
	}
	return pb.writeEpoch(blk)
}
func (pb *PutBlockToTrieDB) writeEpoch(blk *block.Block) error {
	// need write the current and the last epoch height and del the epoch height before the last epoch height
	ctx, err := pb.bc.Context()
	if err != nil {
		return err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochNum := rp.GetEpochNum(blk.Height())
	epochHeight := rp.GetEpochHeight(epochNum)
	epochBlk, err := pb.bc.BlockDAO().GetBlockByHeight(epochHeight)
	if err != nil {
		return err
	}
	if err = pb.writeBlock(epochBlk); err != nil {
		return err
	}

	if epochNum > 1 {
		beforeLastBlkHeight := rp.GetEpochHeight(epochNum - 1)
		beforeLastBlkHeightBlk, err := pb.bc.BlockDAO().GetBlockByHeight(beforeLastBlkHeight)
		if err != nil {
			return err
		}
		if err = pb.writeBlock(beforeLastBlkHeightBlk); err != nil {
			return err
		}
	}
	if epochNum > 2 {
		needDelBlkHeight := rp.GetEpochHeight(epochNum - 2)
		if err = pb.delBlock(needDelBlkHeight); err != nil {
			return err
		}
	}
	return nil
}
func GetTopBlock(kv db.KVStore) (*block.Block, error) {
	heightValue, err := kv.Get(blockTopNS, topBlockKey)
	if err != nil {
		return nil, err
	}
	return GetBlock(kv, heightValue)
}
func GetLastEpochBlock(kv db.KVStore, ctx context.Context, height uint64) (ret []*block.Block, err error) {
	log.L().Info("GetLastEpochBlock:", zap.Uint64("height", height))
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochNum := rp.GetEpochNum(height)
	epochNumHeight := rp.GetEpochHeight(epochNum)
	heightValue := byteutil.Uint64ToBytes(epochNumHeight)
	blk, err := GetBlock(kv, heightValue)
	if err != nil {
		return
	}
	ret = append(ret, blk)
	if epochNum > 1 {
		log.L().Info("epochNum-1", zap.Uint64("epochNum-1", epochNum-1))
		beforeLastBlkHeight := rp.GetEpochHeight(epochNum - 1)
		heightValue := byteutil.Uint64ToBytes(beforeLastBlkHeight)
		blk, err = GetBlock(kv, heightValue)
		if err != nil {
			return
		}
		ret = append(ret, blk)
	}
	return
}
func GetBlock(kv db.KVStore, heightValue []byte) (*block.Block, error) {
	blkSer, err := kv.Get(blockNS, heightValue)
	if err != nil {
		return nil, err
	}
	blk := &block.Block{}
	if err := blk.Deserialize(blkSer); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block")
	}
	receipts, err := kv.Get(blockReceipNS, heightValue)
	if err != nil {
		return nil, err
	}
	receiptsPb := &iotextypes.Receipts{}
	if err := proto.Unmarshal(receipts, receiptsPb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block receipts")
	}
	for _, receiptPb := range receiptsPb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		blk.Receipts = append(blk.Receipts, receipt)
	}
	return blk, nil
}
