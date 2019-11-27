// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"os"
	"path"

	"github.com/iotexproject/iotex-core/action"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-election/util"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

// these NS belong to old DB before migrating to storage optimization
// they are left here only for record
// do NOT use them in the future to avoid potential conflict
const (
	blockHeaderNS = "bhr"
	blockBodyNS   = "bbd"
	blockFooterNS = "bfr"
	receiptsNS    = "rpt"
)

var (
	tipHeightKey = []byte("th")
)

var (
	pattern = "-00000000.db"
)

func (dao *blockDAO) checkLegacyDB() error {
	fileExists := func(path string) bool {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			return false
		}
		if err != nil {
			zap.L().Panic("unexpected error", zap.Error(err))
		}
		return true
	}
	ext := path.Ext(dao.cfg.DbPath)
	var fileName string
	if len(ext) > 0 {
		fileName = dao.cfg.DbPath[:len(dao.cfg.DbPath)-len(ext)] + pattern
	}
	log.L().Info("checkOldDB::", zap.String("fileName", fileName))
	if fileExists(fileName) {
		return nil
	}

	bakDbPath := path.Dir(dao.cfg.DbPath) + "/oldchain.db"
	log.L().Info("bakDbPath::", zap.String("bakDbPath:", bakDbPath))
	if err := os.Rename(dao.cfg.DbPath, bakDbPath); err != nil {
		return err
	}
	cfgDB := dao.cfg
	cfgDB.DbPath = bakDbPath
	bakdb := db.NewBoltDB(cfgDB)
	dao.legacyDB = bakdb
	return nil
}

func (dao *blockDAO) migrate() error {
	if err := dao.legacyDB.Start(context.Background()); err != nil {
		return err
	}
	defer dao.legacyDB.Stop(context.Background())

	tipHeightValue, err := dao.legacyDB.Get(blockNS, tipHeightKey)
	if err != nil {
		return err
	}
	tipHeight := util.BytesToUint64(tipHeightValue)
	log.L().Info("tipHeight:", zap.Uint64("height", tipHeight))
	kvForBlockData, _, err := dao.getTopDB(1)
	if err != nil {
		return err
	}
	if dao.blockIndex, err = db.NewCountingIndexNX(kvForBlockData, []byte(blockDataNS)); err != nil {
		return err
	}
	if dao.receiptIndex, err = db.NewCountingIndexNX(kvForBlockData, []byte(recptDataNS)); err != nil {
		return err
	}
	batch := db.NewBatch()
	blockBatch := db.NewBatch()
	for i := uint64(1); i <= tipHeight; i++ {
		blk, err := dao.getBlockLegacy(i)
		if err != nil {
			return err
		}
		if err = dao.putBlockLegacy(blk, batch, blockBatch, kvForBlockData); err != nil {
			return err
		}
		if i%10000 == 0 || i == tipHeight {
			kvForBlockData, err = dao.commitAndRefresh(i, batch, blockBatch, kvForBlockData)
			if err != nil {
				return err
			}
		}
		if i%100 == 0 {
			log.L().Info("putBlock:", zap.Uint64("height", i))
		}
	}
	return nil
}

func (dao *blockDAO) commitAndRefresh(height uint64, batch, blockBatch db.KVStoreBatch, kv db.KVStore) (kvForBlockData db.KVStore, err error) {
	if err = dao.commitForMigration(batch, blockBatch, kv); err != nil {
		return
	}
	kvForBlockData, _, err = dao.getTopDB(height)
	if err != nil {
		return
	}
	dao.blockIndex, err = db.NewCountingIndexNX(kvForBlockData, []byte(blockDataNS))
	if err != nil {
		return
	}
	dao.receiptIndex, err = db.NewCountingIndexNX(kvForBlockData, []byte(recptDataNS))
	if err != nil {
		return
	}
	return
}

func (dao *blockDAO) commitForMigration(batch, batchForBlock db.KVStoreBatch, kvForBlockData db.KVStore) error {
	if err := dao.blockIndex.Commit(); err != nil {
		return err
	}
	if err := dao.receiptIndex.Commit(); err != nil {
		return err
	}
	if err := dao.kvstore.Commit(batch); err != nil {
		return err
	}
	return kvForBlockData.Commit(batchForBlock)
}

// putBlock puts a block
func (dao *blockDAO) putBlockLegacy(blk *block.Block, batch, blockBatch db.KVStoreBatch, kv db.KVStore) error {
	if err := dao.putBlockForBlockdbLegacy(blk, blockBatch, kv); err != nil {
		return err
	}
	blkHeight := blk.Height()
	hash := blk.HashBlock()
	heightValue := byteutil.Uint64ToBytes(blkHeight)
	hashKey := append(hashPrefix, hash[:]...)
	batch.Put(blockHashHeightMappingNS, hashKey, heightValue, "failed to put hash -> height mapping")
	if err := addHeightHash(dao.kvstore, hash); err != nil {
		return errors.Wrap(err, "failed to add height -> hash mapping")
	}
	topHeight, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get top height")
	}
	if blkHeight > enc.MachineEndian.Uint64(topHeight) {
		batch.Put(blockNS, topHeightKey, heightValue, "failed to put top height")
		batch.Put(blockNS, topHashKey, hash[:], "failed to put top hash")
	}
	return nil
}

func (dao *blockDAO) putBlockForBlockdbLegacy(blk *block.Block, blockBatch db.KVStoreBatch, kv db.KVStore) error {
	blkHeight := blk.Height()
	heightValue := byteutil.Uint64ToBytes(blkHeight)
	h := blk.HashBlock()

	batchForBlock := db.NewBatch()
	_, err := kv.Get(blockNS, startHeightKey)
	if err != nil && errors.Cause(err) == db.ErrNotExist {
		batchForBlock.Put(blockNS, startHeightKey, heightValue, "failed to put start height key")
	}
	serBlk, err := blk.Serialize()
	if err != nil {
		return errors.Wrap(err, "failed to serialize block")
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("compress_header")
		serBlk, err = compress.Compress(serBlk)
		timer.End()
		if err != nil {
			return errors.Wrapf(err, "error when compressing a block")
		}
	}
	if err = dao.blockIndex.Add(serBlk, true); err != nil {
		return err
	}
	if err = addHeightHash(kv, h); err != nil {
		return err
	}
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			if err = dao.receiptIndex.Add(receiptsBytes, true); err != nil {
				return err
			}
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}
	var topHeight uint64
	topHeightValue, err := kv.Get(blockNS, topHeightKey)
	if err != nil {
		topHeight = 0
	} else {
		topHeight = enc.MachineEndian.Uint64(topHeightValue)
	}
	if blkHeight > topHeight {
		blockBatch.Put(blockNS, topHeightKey, heightValue, "failed to put top height")
		blockBatch.Put(blockNS, topHashKey, h[:], "failed to put top hash")
	}
	return nil
}

// getBlockHash returns the block hash by height
func (dao *blockDAO) getLegacyBlockHash(height uint64) (hash.Hash256, error) {
	h := hash.ZeroHash256
	if height == 0 {
		return h, nil
	}
	key := heightKey(height)
	value, err := dao.legacyDB.Get(blockHashHeightMappingNS, key)
	if err != nil {
		return h, errors.Wrap(err, "failed to get block hash")
	}
	if len(h) != len(value) {
		return h, errors.Wrapf(err, "blockhash is broken with length = %d", len(value))
	}
	copy(h[:], value)
	return h, nil
}

// getBlock returns a block
func (dao *blockDAO) getBlockLegacy(height uint64) (*block.Block, error) {
	hash, err := dao.getLegacyBlockHash(height)
	if err != nil {
		return nil, err
	}
	header, err := dao.headerLegacy(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", hash)
	}
	body, err := dao.bodyLegacy(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", hash)
	}
	footer, err := dao.footerLegacy(hash)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", hash)
	}
	receipts, err := dao.receiptsLegacy(height)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block receipts %d", height)
	}
	return &block.Block{
		Header:   *header,
		Body:     *body,
		Footer:   *footer,
		Receipts: receipts,
	}, nil
}

func (dao *blockDAO) headerLegacy(h hash.Hash256) (*block.Header, error) {
	value, err := dao.legacyDB.Get(blockHeaderNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block header %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_header")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block header %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block header %x is missing", h)
	}
	header := &block.Header{}
	if err := header.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block header %x", h)
	}
	return header, nil
}

func (dao *blockDAO) bodyLegacy(h hash.Hash256) (*block.Body, error) {
	value, err := dao.legacyDB.Get(blockBodyNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", h)
	}
	if dao.compressBlock {
		value, err = compress.Decompress(value)
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block body %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block body %x is missing", h)
	}
	body := &block.Body{}
	if err := body.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block body %x", h)
	}
	return body, nil
}

func (dao *blockDAO) footerLegacy(h hash.Hash256) (*block.Footer, error) {
	value, err := dao.legacyDB.Get(blockFooterNS, h[:])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block footer %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_footer")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block footer %x", h)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block footer %x is missing", h)
	}
	footer := &block.Footer{}
	if err := footer.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block footer %x", h)
	}
	return footer, nil
}

func (dao *blockDAO) receiptsLegacy(blkHeight uint64) ([]*action.Receipt, error) {
	value, err := dao.legacyDB.Get(receiptsNS, byteutil.Uint64ToBytes(blkHeight))
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get receipts of block %d", blkHeight)
	}
	if len(value) == 0 {
		return nil, errors.Wrap(db.ErrNotExist, "block receipts missing")
	}
	receiptsPb := &iotextypes.Receipts{}
	if err := proto.Unmarshal(value, receiptsPb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal block receipts")
	}
	var blockReceipts []*action.Receipt
	for _, receiptPb := range receiptsPb.Receipts {
		receipt := &action.Receipt{}
		receipt.ConvertFromReceiptPb(receiptPb)
		blockReceipts = append(blockReceipts, receipt)
	}
	return blockReceipts, nil
}
