// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockchain

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/iotex-core/action"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/state/factory"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

var (
	blockNS       = "blockNS"
	topBlockKey   = []byte("tbk")
	topReceiptKey = []byte("trk")
)

type PutBlockToTrieDB struct {
	sf factory.Factory
}

func NewPutBlockToTrieDB(sf factory.Factory) *PutBlockToTrieDB {
	p := &PutBlockToTrieDB{}
	p.sf = sf
	return p
}
func (pb *PutBlockToTrieDB) HandleBlock(blk *block.Block) error {
	ws, err := pb.sf.NewWorkingSet()
	if err != nil {
		return err
	}
	kv := ws.GetDB()
	blkHeight := blk.Height()
	blkSer, err := blk.Serialize()
	batch := db.NewBatch()
	batch.Put(blockNS, topBlockKey, blkSer, "failed to put block")
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			batch.Put(blockNS, topReceiptKey, receiptsBytes, "failed to put receipts")
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}
	return kv.Commit(batch)
}

func GetTopBlock(kv db.KVStore) (*block.Block, error) {
	blkSer, err := kv.Get(blockNS, topBlockKey)
	if err != nil {
		return nil, err
	}
	blk := &block.Block{}
	if err := blk.Deserialize(blkSer); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block")
	}
	receipts, err := kv.Get(blockNS, topReceiptKey)
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
