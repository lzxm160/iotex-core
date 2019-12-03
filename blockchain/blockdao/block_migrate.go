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

	"github.com/iotexproject/iotex-election/util"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/byteutil"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func (dao *blockDAO) isLegacyDB() bool {
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

	return fileExists(fileName)
}

func (dao *blockDAO) initMigrate() error {
	bakDbPath := path.Dir(dao.cfg.DbPath) + "/oldchain.db"
	log.L().Info("bakDbPath::", zap.String("bakDbPath:", bakDbPath))
	if err := os.Rename(dao.cfg.DbPath, bakDbPath); err != nil {
		return err
	}
	cfgDB := dao.cfg
	cfgDB.DbPath = bakDbPath
	dao.kvstore = db.NewBoltDB(cfgDB)
	tipHeight, err := dao.getTipHeight()
	if err != nil {
		return err
	}
	kv, _, err := dao.getTopDB(tipHeight)
	if err != nil {
		return err
	}
	if dao.blkStore, err = db.NewCountingIndexNX(kv, []byte(blockDataNS)); err != nil {
		return err
	}
	if dao.blkStore.Size() == 0 {
		if err = dao.blkStore.Add(make([]byte, 0), false); err != nil {
			return err
		}
	}
	if dao.receiptStore, err = db.NewCountingIndexNX(kv, []byte(recptDataNS)); err != nil {
		return err
	}
	if dao.receiptStore.Size() == 0 {
		if err = dao.receiptStore.Add(make([]byte, 0), false); err != nil {
			return err
		}
	}
	if dao.hashStore, err = db.NewCountingIndexNX(kv, []byte(hashDataNS)); err != nil {
		return err
	}
	if dao.hashStore.Size() == 0 {
		if err = dao.hashStore.Add(make([]byte, 0), false); err != nil {
			return err
		}
	}
	return nil
}

func (dao *blockDAO) migrate() error {
	cfg := dao.cfg
	legacyDB := db.NewBoltDB(cfg)
	if err := legacyDB.Start(context.Background()); err != nil {
		return err
	}
	defer legacyDB.Stop(context.Background())

	tipHeightValue, err := dao.kvstore.Get(blockNS, tipHeightKey)
	if err != nil {
		return err
	}
	tipHeight := util.BytesToUint64(tipHeightValue)
	log.L().Info("tipHeight:", zap.Uint64("height", tipHeight))
	for i := uint64(1); i <= tipHeight; i++ {
		blk, err := dao.getBlockByHeightLegacy(i)
		if err != nil {
			return err
		}
		if err = dao.putBlockForMigration(blk); err != nil {
			return err
		}
		if i%5000 == 0 || i == tipHeight {
			err = dao.commitForMigration(legacyDB)
			if err != nil {
				return err
			}
		}
		if i%100 == 0 {
			log.L().Info("putBlock:", zap.Uint64("height", i))
		}
	}
	dao.kvstore = legacyDB
	return os.Remove(path.Dir(dao.cfg.DbPath) + "/oldchain.db")
}

func (dao *blockDAO) getBlockByHeightLegacy(height uint64) (*block.Block, error) {
	h, err := dao.getBlockHashLegacy(height)
	if err != nil {
		return nil, err
	}
	blk, err := dao.getBlockLegacy(h)
	if err != nil {
		return nil, err
	}
	receipts, err := dao.getReceiptsLegacy(height)
	if err != nil {
		return nil, err
	}
	blk.Receipts = receipts
	return blk, nil
}

func (dao *blockDAO) commitForMigration(kvstore db.KVStore) error {
	if err := dao.blkStore.Commit(); err != nil {
		return err
	}
	if err := dao.receiptStore.Commit(); err != nil {
		return err
	}
	if err := dao.hashStore.Commit(); err != nil {
		return err
	}
	return kvstore.Commit(dao.batch)
}

func (dao *blockDAO) putBlockForMigration(blk *block.Block) error {
	blkHeight := blk.Height()
	h, err := dao.getBlockHashLegacy(blkHeight)
	if h != hash.ZeroHash256 && err == nil {
		return errors.Errorf("block %d already exist", blkHeight)
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
	if err = dao.blkStore.Add(serBlk, true); err != nil {
		return err
	}
	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			if err = dao.receiptStore.Add(receiptsBytes, true); err != nil {
				return err
			}
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}
	h = blk.HashBlock()
	if err = dao.hashStore.Add(h[:], true); err != nil {
		return nil
	}

	heightValue := byteutil.Uint64ToBytes(blkHeight)
	hashKey := hashKey(h)
	dao.batch.Put(blockHashHeightMappingNS, hashKey, heightValue, "failed to put hash -> height mapping")
	tipHeight, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get top height")
	}
	if blkHeight > enc.MachineEndian.Uint64(tipHeight) {
		dao.batch.Put(blockNS, topHeightKey, heightValue, "failed to put top height")
		dao.batch.Put(blockNS, topHashKey, h[:], "failed to put top hash")
	}
	return nil
}

func (dao *blockDAO) getBlockHashLegacy(height uint64) (hash.Hash256, error) {
	h := hash.ZeroHash256
	if height == 0 {
		return h, nil
	}
	key := heightKey(height)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
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
	value, err := dao.getBlockValue(blockHeaderNS, h)
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
	value, err := dao.getBlockValue(blockBodyNS, h)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block body %x", h)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_body")
		value, err = compress.Decompress(value)
		timer.End()
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
	value, err := dao.getBlockValue(blockFooterNS, h)
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

// getDBFromHash returns db of this block stored
func (dao *blockDAO) getDBFromHash(h hash.Hash256) (db.KVStore, uint64, error) {
	height, err := dao.getBlockHeight(h)
	if err != nil {
		return nil, 0, err
	}
	return dao.getDBFromHeight(height)
}

// getBlockValue get block's data from db,if this db failed,it will try the previous one
func (dao *blockDAO) getBlockValue(blockNS string, h hash.Hash256) ([]byte, error) {
	whichDB, index, err := dao.getDBFromHash(h)
	if err != nil {
		return nil, err
	}
	value, err := whichDB.Get(blockNS, h[:])
	if errors.Cause(err) == db.ErrNotExist {
		idx := index - 1
		if index == 0 {
			idx = 0
		}
		db, _, err := dao.getDBFromIndex(idx)
		if err != nil {
			return nil, err
		}
		value, err = db.Get(blockNS, h[:])
		if err != nil {
			return nil, err
		}
	}
	return value, err
}

func (dao *blockDAO) getReceiptsLegacy(blkHeight uint64) ([]*action.Receipt, error) {
	kvstore, _, err := dao.getDBFromHeight(blkHeight)
	if err != nil {
		return nil, err
	}
	value, err := kvstore.Get(receiptsNS, byteutil.Uint64ToBytes(blkHeight))
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
