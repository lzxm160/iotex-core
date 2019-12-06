// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"context"
	"encoding/hex"

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
func (pb *PutBlockToTrieDB) writeBlock(blk *block.Block, key []byte) error {
	ws, err := pb.bc.Factory().NewWorkingSet()
	if err != nil {
		return err
	}
	kv := ws.GetDB()
	blkHeight := blk.Height()
	blkSer, err := blk.Serialize()
	batch := db.NewBatch()

	batch.Put(blockNS, key, blkSer, "failed to put block")
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			batch.Put(blockReceipNS, key, receiptsBytes, "failed to put receipts")
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
	heightKey := byteutil.Uint64ToBytes(height)
	batch := db.NewBatch()
	batch.Delete(blockNS, heightKey, "failed to del block")
	batch.Delete(blockReceipNS, heightKey, "failed to del receipts")
	return kv.Commit(batch)
}
func (pb *PutBlockToTrieDB) writeTopBlock(blk *block.Block) error {
	return pb.writeBlock(blk, topBlockKey)
}
func (pb *PutBlockToTrieDB) HandleBlock(blk *block.Block) error {
	err := pb.writeTopBlock(blk)
	if err != nil {
		return err
	}
	ctx, err := pb.bc.Context()
	if err != nil {
		return err
	}
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochNum := rp.GetEpochNum(blk.Height())
	epochHeight := rp.GetEpochHeight(epochNum)
	if epochHeight == blk.Height() {
		return pb.writeEpoch(blk, epochNum, rp)
	}
	return nil
}
func (pb *PutBlockToTrieDB) writeEpoch(blk *block.Block, epochNum uint64, rp *rolldpos.Protocol) error {
	// need write the current and the last epoch height and del the epoch height before the last epoch height
	epochHeight := blk.Height()
	epochBlk, err := pb.bc.BlockDAO().GetBlockByHeight(epochHeight)
	if err != nil {
		return err
	}
	epochBlk.Receipts, err = pb.bc.BlockDAO().GetReceipts(epochHeight)
	if err != nil {
		return err
	}

	heightKey := byteutil.Uint64ToBytes(epochBlk.Height())
	log.L().Info("writeEpoch:", zap.Uint64("blk.Height()", blk.Height()), zap.Uint64("epochHeight", epochHeight), zap.String("heightKey", hex.EncodeToString(heightKey)))
	if err = pb.writeBlock(epochBlk, heightKey); err != nil {
		return err
	}

	if epochHeight > 1 {
		epochHeight--
		epochBlk, err := pb.bc.BlockDAO().GetBlockByHeight(epochHeight)
		if err != nil {
			return err
		}
		epochBlk.Receipts, err = pb.bc.BlockDAO().GetReceipts(epochHeight)
		if err != nil {
			return err
		}

		heightKey := byteutil.Uint64ToBytes(epochBlk.Height())
		log.L().Info("writeEpoch:", zap.Uint64("blk.Height()", blk.Height()), zap.Uint64("epochHeight", epochHeight), zap.String("heightKey", hex.EncodeToString(heightKey)))
		if err = pb.writeBlock(epochBlk, heightKey); err != nil {
			return err
		}
	}

	if epochNum > 1 {
		beforeLastBlkHeight := rp.GetEpochHeight(epochNum - 1)
		beforeLastBlkHeightBlk, err := pb.bc.BlockDAO().GetBlockByHeight(beforeLastBlkHeight)
		if err != nil {
			return err
		}
		beforeLastBlkHeightBlk.Receipts, err = pb.bc.BlockDAO().GetReceipts(beforeLastBlkHeight)
		if err != nil {
			return err
		}
		heightKey = byteutil.Uint64ToBytes(beforeLastBlkHeightBlk.Height())
		log.L().Info("writeEpoch:", zap.Uint64("beforeLastBlkHeightBlk.Height()", beforeLastBlkHeightBlk.Height()), zap.Uint64("beforeLastBlkHeight", beforeLastBlkHeight), zap.String("heightKey", hex.EncodeToString(heightKey)))
		if err = pb.writeBlock(beforeLastBlkHeightBlk, heightKey); err != nil {
			return err
		}

		if beforeLastBlkHeight > 1 {
			beforeLastBlkHeight--
			beforeLastBlkHeightBlk, err := pb.bc.BlockDAO().GetBlockByHeight(beforeLastBlkHeight)
			if err != nil {
				return err
			}
			beforeLastBlkHeightBlk.Receipts, err = pb.bc.BlockDAO().GetReceipts(beforeLastBlkHeight)
			if err != nil {
				return err
			}
			heightKey = byteutil.Uint64ToBytes(beforeLastBlkHeightBlk.Height())
			log.L().Info("writeEpoch:", zap.Uint64("beforeLastBlkHeightBlk.Height()", beforeLastBlkHeightBlk.Height()), zap.Uint64("beforeLastBlkHeight", beforeLastBlkHeight), zap.String("heightKey", hex.EncodeToString(heightKey)))
			if err = pb.writeBlock(beforeLastBlkHeightBlk, heightKey); err != nil {
				return err
			}
		}
	}
	if epochNum > 2 {
		needDelBlkHeight := rp.GetEpochHeight(epochNum - 2)
		log.L().Info("writeEpoch:", zap.Uint64("needDelBlkHeight", needDelBlkHeight))
		if err = pb.delBlock(needDelBlkHeight); err != nil {
			return err
		}
		if needDelBlkHeight > 1 {
			if err = pb.delBlock(needDelBlkHeight - 1); err != nil {
				return err
			}
		}
	}
	return nil
}
func GetTopBlock(kv db.KVStore) (*block.Block, error) {
	return GetBlock(kv, topBlockKey)
}
func GetLastEpochBlock(kv db.KVStore, ctx context.Context, height uint64) (ret []*block.Block, err error) {
	bcCtx := protocol.MustGetBlockchainCtx(ctx)
	rp := rolldpos.MustGetProtocol(bcCtx.Registry)
	epochNum := rp.GetEpochNum(height)
	epochHeight := rp.GetEpochHeight(epochNum)

	heightKey := byteutil.Uint64ToBytes(epochHeight)
	log.L().Info("GetLastEpochBlock:", zap.Uint64("height", height), zap.Uint64("epochHeight", epochHeight), zap.String("heightKey", hex.EncodeToString(heightKey)))
	blk, err := GetBlock(kv, heightKey)
	if err != nil {
		return
	}
	ret = append(ret, blk)
	if epochNum > 1 {
		lastEpochHeight := rp.GetEpochHeight(epochNum - 1)
		heightKey = byteutil.Uint64ToBytes(lastEpochHeight)

		log.L().Info("epochNum-1", zap.Uint64("epochNum-1", epochNum-1), zap.Uint64("lastEpochHeight", lastEpochHeight), zap.String("heightKey", hex.EncodeToString(heightKey)))

		blk, err = GetBlock(kv, heightKey)
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
