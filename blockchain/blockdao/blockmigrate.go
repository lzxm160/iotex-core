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

const (
	blockHeaderNS = "bhr"
	blockBodyNS   = "bbd"
	blockFooterNS = "bfr"
)

var (
	tipHeightKey = []byte("th")
)

var (
	pattern = "-00000000.db"
)

func (dao *blockDAO) checkOldDB() error {
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
	dao.oldDB = bakdb
	return nil
}

func (dao *blockDAO) migrate() error {
	if err := dao.oldDB.Start(context.Background()); err != nil {
		return err
	}
	defer dao.oldDB.Stop(context.Background())

	tipHeightValue, err := dao.oldDB.Get(blockNS, tipHeightKey)
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
	if dao.receiptIndex, err = db.NewCountingIndexNX(kvForBlockData, []byte(receiptsNS)); err != nil {
		return err
	}
	batch := db.NewBatch()
	blockBatch := db.NewBatch()
	for i := uint64(1); i <= tipHeight; i++ {
		h, err := dao.getLegacyBlockHash(i)
		if err != nil {
			return err
		}
		blk, err := dao.getBlockLegacy(h)
		if err != nil {
			return err
		}
		if err = dao.putBlockLegacy(blk, batch, blockBatch, kvForBlockData); err != nil {
			return err
		}
		if i%10000 == 0 {
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
	dao.receiptIndex, err = db.NewCountingIndexNX(kvForBlockData, []byte(receiptsNS))
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
	heightKey := heightKey(blkHeight)
	hashKey := append(hashPrefix, hash[:]...)
	batch.Put(blockHashHeightMappingNS, hashKey, heightValue, "failed to put hash -> height mapping")
	batch.Put(blockHashHeightMappingNS, heightKey, hash[:], "failed to put height -> hash mapping")
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

	heightKey := heightKey(blkHeight)
	batchForBlock.Put(blockHashHeightMappingNS, heightKey, h[:], "failed to put height -> hash mapping")
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
	value, err := dao.oldDB.Get(blockHashHeightMappingNS, key)
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
func (dao *blockDAO) getBlockLegacy(hash hash.Hash256) (*block.Block, error) {
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
	return &block.Block{
		Header: *header,
		Body:   *body,
		Footer: *footer,
	}, nil
}

func (dao *blockDAO) headerLegacy(h hash.Hash256) (*block.Header, error) {
	value, err := dao.oldDB.Get(blockHeaderNS, h[:])
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
	value, err := dao.oldDB.Get(blockBodyNS, h[:])
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
	value, err := dao.oldDB.Get(blockFooterNS, h[:])
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
