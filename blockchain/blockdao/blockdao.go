// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package blockdao

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/golang/protobuf/proto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/cache"
	"github.com/iotexproject/iotex-core/pkg/compress"
	"github.com/iotexproject/iotex-core/pkg/enc"
	"github.com/iotexproject/iotex-core/pkg/lifecycle"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/prometheustimer"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

const (
	blockNS                  = "blk"
	blockHashHeightMappingNS = "h2h"
	blockDataNS              = "bdn"
	recptDataNS              = "rct"
	hashDataNS               = "hsh"
)

// these NS belong to old DB before migrating to separate index
// they are left here only for record
// do NOT use them in the future to avoid potential conflict
const (
	blockActionBlockMappingNS        = "a2b"
	blockAddressActionMappingNS      = "a2a"
	blockAddressActionCountMappingNS = "a2c"
	blockActionReceiptMappingNS      = "a2r"
	numActionsNS                     = "nac"
	transferAmountNS                 = "tfa"
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
	topHeightKey       = []byte("th")
	startHeightKey     = []byte("sh")
	topHashKey         = []byte("ts")
	hashPrefix         = []byte("ha.")
	heightPrefix       = []byte("he.")
	heightToFileBucket = []byte("h2f")
	tipHeightKey       = []byte("th")
)

var (
	cacheMtc = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "iotex_blockdao_cache",
			Help: "IoTeX blockdao cache counter.",
		},
		[]string{"result"},
	)
	patternLen = len("00000000.db")
	pattern    = "-00000000.db"
	suffixLen  = len(".db")
	// ErrNotOpened indicates db is not opened
	ErrNotOpened = errors.New("DB is not opened")
	// ErrMissingBlock indicates block db is missing blocks
	ErrMissingBlock = errors.New("block db is missing block")
)

type (
	// BlockDAO represents the block data access object
	BlockDAO interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		GetBlockHash(uint64) (hash.Hash256, error)
		GetBlockHeight(hash.Hash256) (uint64, error)
		GetBlock(hash.Hash256) (*block.Block, error)
		GetBlockByHeight(uint64) (*block.Block, error)
		GetTipHeight() (uint64, error)
		GetTipHash() (hash.Hash256, error)
		Header(hash.Hash256) (*block.Header, error)
		Body(hash.Hash256) (*block.Body, error)
		Footer(hash.Hash256) (*block.Footer, error)
		GetActionByActionHash(hash.Hash256, uint64) (action.SealedEnvelope, error)
		GetReceiptByActionHash(hash.Hash256, uint64) (*action.Receipt, error)
		GetReceipts(uint64) ([]*action.Receipt, error)
		PutBlock(*block.Block) error
		Commit() error
		DeleteBlockToTarget(uint64) error
		IndexFile(uint64, []byte) error
		GetFileIndex(uint64) ([]byte, error)
		KVStore() db.KVStore
	}

	// BlockIndexer defines an interface to accept block to build index
	BlockIndexer interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		PutBlock(blk *block.Block) error
		DeleteTipBlock(blk *block.Block) error
		Commit() error
	}

	blockDAO struct {
		compressBlock bool
		kvstore       db.KVStore
		indexer       BlockIndexer
		htf           db.RangeIndex
		hashStore     db.CountingIndex
		kvstores      sync.Map //store like map[index]db.KVStore,index from 1...N
		topIndex      atomic.Value
		timerFactory  *prometheustimer.TimerFactory
		lifecycle     lifecycle.Lifecycle
		headerCache   *cache.ThreadSafeLruCache
		bodyCache     *cache.ThreadSafeLruCache
		footerCache   *cache.ThreadSafeLruCache
		cfg           config.DB
		mutex         sync.RWMutex // for create new db file
		// TODO: delete below after reasonably long period of time passed DB migration
		blkStore     db.CountingIndex
		receiptStore db.CountingIndex
		batch        db.KVStoreBatch
	}
)

// NewBlockDAO instantiates a block DAO
func NewBlockDAO(kvstore db.KVStore, indexer BlockIndexer, compressBlock bool, cfg config.DB) BlockDAO {
	blockDAO := &blockDAO{
		compressBlock: compressBlock,
		kvstore:       kvstore,
		indexer:       indexer,
		cfg:           cfg,
		batch:         db.NewBatch(),
	}
	if cfg.MaxCacheSize > 0 {
		blockDAO.headerCache = cache.NewThreadSafeLruCache(cfg.MaxCacheSize)
		blockDAO.bodyCache = cache.NewThreadSafeLruCache(cfg.MaxCacheSize)
		blockDAO.footerCache = cache.NewThreadSafeLruCache(cfg.MaxCacheSize)
	}
	timerFactory, err := prometheustimer.New(
		"iotex_block_dao_perf",
		"Performance of block DAO",
		[]string{"type"},
		[]string{"default"},
	)
	if err != nil {
		return nil
	}
	blockDAO.timerFactory = timerFactory
	if indexer != nil {
		blockDAO.lifecycle.Add(indexer)
	}
	// check if have old db
	if blockDAO.isLegacyDB() {
		blockDAO.lifecycle.Add(kvstore)
	}
	return blockDAO
}

// Start starts block DAO and initiates the top height if it doesn't exist
func (dao *blockDAO) Start(ctx context.Context) error {
	if !dao.isLegacyDB() {
		// have to check and init here,because the following code will open chaindb
		err := dao.initMigrate()
		if err != nil {
			return nil
		}
		dao.migrate()
	}
	err := dao.lifecycle.OnStart(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start child services")
	}
	// set init height value
	if _, err = dao.kvstore.Get(blockNS, topHeightKey); err != nil &&
		errors.Cause(err) == db.ErrNotExist {
		if err := dao.kvstore.Put(blockNS, topHeightKey, make([]byte, 8)); err != nil {
			return errors.Wrap(err, "failed to write initial value for top height")
		}
	}

	if err = dao.initCountingIndex(); err != nil {
		return err
	}
	if err = dao.initStores(); err != nil {
		return err
	}

	return nil
}

func (dao *blockDAO) initStores() error {
	cfg := dao.cfg
	model, dir := getFileNameAndDir(cfg.DbPath)
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	var maxN uint64
	for _, file := range files {
		name := file.Name()
		lens := len(name)
		if lens < patternLen || !strings.Contains(name, model) {
			continue
		}
		num := name[lens-patternLen : lens-suffixLen]
		n, err := strconv.Atoi(num)
		if err != nil {
			continue
		}
		dao.openDB(uint64(n))
		if uint64(n) > maxN {
			maxN = uint64(n)
		}
	}
	dao.topIndex.Store(maxN)
	return nil
}

func (dao *blockDAO) initCountingIndex() error {
	var err error
	if dao.hashStore, err = db.NewCountingIndexNX(dao.kvstore, []byte(hashDataNS)); err != nil {
		return err
	}
	if dao.hashStore.Size() == 0 {
		return dao.hashStore.Add(hash.ZeroHash256[:], false)
	}
	return nil
}

func (dao *blockDAO) Stop(ctx context.Context) error { return dao.lifecycle.OnStop(ctx) }

func (dao *blockDAO) Commit() error {
	return nil
}

func (dao *blockDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	return dao.getBlockHash(height)
}

func (dao *blockDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	return dao.getBlockHeight(hash)
}

func (dao *blockDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	return dao.getBlock(hash)
}

func (dao *blockDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	return dao.getBlockByHeight(height)
}

func (dao *blockDAO) GetTipHash() (hash.Hash256, error) {
	return dao.getTipHash()
}

func (dao *blockDAO) GetTipHeight() (uint64, error) {
	return dao.getTipHeight()
}

func (dao *blockDAO) Header(h hash.Hash256) (*block.Header, error) {
	return dao.header(h)
}

func (dao *blockDAO) Body(h hash.Hash256) (*block.Body, error) {
	return dao.body(h)
}

func (dao *blockDAO) Footer(h hash.Hash256) (*block.Footer, error) {
	return dao.footer(h)
}

func (dao *blockDAO) GetActionByActionHash(h hash.Hash256, height uint64) (action.SealedEnvelope, error) {
	bh, err := dao.getBlockHash(height)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	blk, err := dao.body(bh)
	if err != nil {
		return action.SealedEnvelope{}, err
	}
	for _, act := range blk.Actions {
		if act.Hash() == h {
			return act, nil
		}
	}
	return action.SealedEnvelope{}, errors.Errorf("block %d does not have action %x", height, h)
}

func (dao *blockDAO) GetReceiptByActionHash(h hash.Hash256, height uint64) (*action.Receipt, error) {
	receipts, err := dao.getReceipts(height)
	if err != nil {
		return nil, err
	}
	for _, r := range receipts {
		if r.ActionHash == h {
			return r, nil
		}
	}
	return nil, errors.Errorf("receipt of action %x isn't found", h)
}

func (dao *blockDAO) GetReceipts(blkHeight uint64) ([]*action.Receipt, error) {
	return dao.getReceipts(blkHeight)
}

func (dao *blockDAO) PutBlock(blk *block.Block) error {
	if err := dao.putBlock(blk); err != nil {
		return err
	}
	// index the block if there's indexer
	if dao.indexer == nil {
		return nil
	}
	if err := dao.indexer.PutBlock(blk); err != nil {
		return err
	}
	return dao.indexer.Commit()
}

func (dao *blockDAO) DeleteBlockToTarget(targetHeight uint64) error {
	dao.mutex.Lock()
	defer dao.mutex.Unlock()
	tipHeight, err := dao.getTipHeight()
	if err != nil {
		return err
	}
	for tipHeight > targetHeight {
		// Obtain tip block hash
		h, err := dao.getTipHash()
		if err != nil {
			return errors.Wrap(err, "failed to get tip block hash")
		}
		blk, err := dao.getBlock(h)
		if err != nil {
			return errors.Wrap(err, "failed to get tip block")
		}
		// delete block index if there's indexer
		if dao.indexer != nil {
			if err := dao.indexer.DeleteTipBlock(blk); err != nil {
				return err
			}
		}
		if err := dao.deleteTipBlock(); err != nil {
			return err
		}
		tipHeight--
	}
	return nil
}

func (dao *blockDAO) IndexFile(height uint64, index []byte) error {
	dao.mutex.Lock()
	defer dao.mutex.Unlock()

	if dao.htf == nil {
		htf, err := db.NewRangeIndex(dao.kvstore, heightToFileBucket, make([]byte, 8))
		if err != nil {
			return err
		}
		dao.htf = htf
	}
	return dao.htf.Insert(height, index)
}

// GetFileIndex return the db filename
func (dao *blockDAO) GetFileIndex(height uint64) ([]byte, error) {
	dao.mutex.RLock()
	defer dao.mutex.RUnlock()

	if dao.htf == nil {
		htf, err := db.NewRangeIndex(dao.kvstore, heightToFileBucket, make([]byte, 8))
		if err != nil {
			return nil, err
		}
		dao.htf = htf
	}
	return dao.htf.Get(height)
}

func (dao *blockDAO) KVStore() db.KVStore {
	return dao.kvstore
}

// getBlockHash returns the block hash by height
func (dao *blockDAO) getBlockHash(height uint64) (hash.Hash256, error) {
	h, err := dao.hashStore.Get(height)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to get block hash")
	}
	return hash.BytesToHash256(h), nil
}

// getBlockHeight returns the block height by hash
func (dao *blockDAO) getBlockHeight(hash hash.Hash256) (uint64, error) {
	key := hashKey(hash)
	value, err := dao.kvstore.Get(blockHashHeightMappingNS, key)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get block height")
	}
	if len(value) == 0 {
		return 0, errors.Wrapf(db.ErrNotExist, "height missing for block with hash = %x", hash)
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getBlock returns a block
func (dao *blockDAO) getBlock(h hash.Hash256) (*block.Block, error) {
	height, err := dao.getBlockHeight(h)
	if err != nil {
		return nil, err
	}
	return dao.getBlockByHeight(height)
}

// getBlockByHeight returns a block by height
func (dao *blockDAO) getBlockByHeight(height uint64) (*block.Block, error) {
	whichDB, index, err := dao.getDBFromHeight(height)
	if err != nil {
		return nil, err
	}
	blkStore, err := db.GetCountingIndex(whichDB, []byte(blockDataNS))
	if err != nil {
		return nil, err
	}
	value, err := blkStore.Get(height)
	if errors.Cause(err) == db.ErrNotExist {
		idx := index - 1
		if index == 0 {
			idx = 0
		}
		dbs, _, err := dao.getDBFromIndex(idx)
		if err != nil {
			return nil, err
		}
		if blkStore, err = db.GetCountingIndex(dbs, []byte(blockDataNS)); err != nil {
			return nil, err
		}
		if value, err = blkStore.Get(height); err != nil {
			return nil, errors.Wrapf(err, "failed to get block %d", height)
		}
	}
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get block %d", height)
	}
	if dao.compressBlock {
		timer := dao.timerFactory.NewTimer("decompress_header")
		value, err = compress.Decompress(value)
		timer.End()
		if err != nil {
			return nil, errors.Wrapf(err, "error when decompressing a block %d", height)
		}
	}
	if len(value) == 0 {
		return nil, errors.Wrapf(db.ErrNotExist, "block %d is missing", height)
	}
	blk := &block.Block{}
	if err := blk.Deserialize(value); err != nil {
		return nil, errors.Wrapf(err, "failed to deserialize block %d", height)
	}
	return blk, nil
}

func (dao *blockDAO) header(h hash.Hash256) (*block.Header, error) {
	if dao.headerCache != nil {
		header, ok := dao.headerCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_header").Inc()
			return header.(*block.Header), nil
		}
		cacheMtc.WithLabelValues("miss_header").Inc()
	}
	blk, err := dao.getBlock(h)
	if err != nil {
		return nil, err
	}
	if dao.headerCache != nil {
		dao.headerCache.Add(h, &blk.Header)
	}
	return &blk.Header, nil
}

func (dao *blockDAO) body(h hash.Hash256) (*block.Body, error) {
	if dao.bodyCache != nil {
		body, ok := dao.bodyCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_body").Inc()
			return body.(*block.Body), nil
		}
		cacheMtc.WithLabelValues("miss_body").Inc()
	}
	blk, err := dao.getBlock(h)
	if err != nil {
		return nil, err
	}
	if dao.bodyCache != nil {
		dao.bodyCache.Add(h, &blk.Body)
	}
	return &blk.Body, nil
}

func (dao *blockDAO) footer(h hash.Hash256) (*block.Footer, error) {
	if dao.footerCache != nil {
		footer, ok := dao.footerCache.Get(h)
		if ok {
			cacheMtc.WithLabelValues("hit_footer").Inc()
			return footer.(*block.Footer), nil
		}
		cacheMtc.WithLabelValues("miss_footer").Inc()
	}
	blk, err := dao.getBlock(h)
	if err != nil {
		return nil, err
	}
	if dao.footerCache != nil {
		dao.footerCache.Add(h, &blk.Footer)
	}
	return &blk.Footer, nil
}

// getTipHeight returns the blockchain height
func (dao *blockDAO) getTipHeight() (uint64, error) {
	value, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return 0, errors.Wrap(err, "failed to get top height")
	}
	if len(value) == 0 {
		return 0, errors.Wrap(db.ErrNotExist, "blockchain height missing")
	}
	return enc.MachineEndian.Uint64(value), nil
}

// getTipHash returns the blockchain tip hash
func (dao *blockDAO) getTipHash() (hash.Hash256, error) {
	value, err := dao.kvstore.Get(blockNS, topHashKey)
	if err != nil {
		return hash.ZeroHash256, errors.Wrap(err, "failed to get tip hash")
	}
	return hash.BytesToHash256(value), nil
}

func (dao *blockDAO) getReceipts(blkHeight uint64) ([]*action.Receipt, error) {
	kvstore, _, err := dao.getDBFromHeight(blkHeight)
	if err != nil {
		return nil, err
	}
	receiptStore, err := db.GetCountingIndex(kvstore, []byte(recptDataNS))
	if err != nil {
		return nil, err
	}
	value, err := receiptStore.Get(blkHeight)
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

// putBlock puts a block
func (dao *blockDAO) putBlock(blk *block.Block) error {
	blkHeight := blk.Height()
	h, err := dao.getBlockHash(blkHeight)
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
	kv, _, err := dao.getTopDB(blkHeight)
	if err != nil {
		return err
	}
	blkStore, err := db.NewCountingIndexNX(kv, []byte(blockDataNS))
	if err != nil {
		return err
	}
	if err = blkStore.Add(serBlk, false); err != nil {
		return err
	}

	// write receipts
	if blk.Receipts != nil {
		receipts := iotextypes.Receipts{}
		for _, r := range blk.Receipts {
			receipts.Receipts = append(receipts.Receipts, r.ConvertToReceiptPb())
		}
		if receiptsBytes, err := proto.Marshal(&receipts); err == nil {
			receiptStore, err := db.NewCountingIndexNX(kv, []byte(recptDataNS))
			if err != nil {
				return err
			}
			if err = receiptStore.Add(receiptsBytes, false); err != nil {
				return err
			}
		} else {
			log.L().Error("failed to serialize receipits for block", zap.Uint64("height", blkHeight))
		}
	}

	h = blk.HashBlock()
	hashStore, err := db.NewCountingIndexNX(dao.kvstore, []byte(hashDataNS))
	if err != nil {
		return err
	}
	if err = hashStore.Add(h[:], false); err != nil {
		return err
	}

	batch := db.NewBatch()
	heightValue := byteutil.Uint64ToBytes(blkHeight)
	hashKey := hashKey(h)
	batch.Put(blockHashHeightMappingNS, hashKey, heightValue, "failed to put hash -> height mapping")
	tipHeight, err := dao.kvstore.Get(blockNS, topHeightKey)
	if err != nil {
		return errors.Wrap(err, "failed to get top height")
	}
	if blkHeight > enc.MachineEndian.Uint64(tipHeight) {
		batch.Put(blockNS, topHeightKey, heightValue, "failed to put top height")
		batch.Put(blockNS, topHashKey, h[:], "failed to put top hash")
	}
	return dao.kvstore.Commit(batch)
}

// deleteTipBlock deletes the tip block
func (dao *blockDAO) deleteTipBlock() error {
	// First obtain tip height from db
	height, err := dao.getTipHeight()
	if err != nil {
		return errors.Wrap(err, "failed to get tip height")
	}
	if height == 0 {
		// should not delete genesis block
		return errors.New("cannot delete genesis block")
	}
	// Obtain tip block hash
	hash, err := dao.getTipHash()
	if err != nil {
		return errors.Wrap(err, "failed to get tip block hash")
	}

	batch := db.NewBatch()
	whichDB, _, err := dao.getDBFromHeight(height)
	if err != nil {
		return err
	}
	blkStore, err := db.GetCountingIndex(whichDB, []byte(blockDataNS))
	if err != nil {
		return err
	}
	if blkStore.Size() > 0 {
		if err = blkStore.Revert(1); err != nil {
			return err
		}
	}

	receiptStore, err := db.GetCountingIndex(whichDB, []byte(recptDataNS))
	if err != nil {
		return err
	}
	if receiptStore.Size() > 0 {
		if err = receiptStore.Revert(1); err != nil {
			return err
		}
	}

	if dao.hashStore.Size() > 0 {
		if err = dao.hashStore.Revert(1); err != nil {
			return err
		}
	}
	// Delete hash -> height mapping
	hashKey := hashKey(hash)
	batch.Delete(blockHashHeightMappingNS, hashKey, "failed to delete hash -> height mapping")

	// Update tip height
	batch.Put(blockNS, topHeightKey, byteutil.Uint64ToBytes(height-1), "failed to put top height")

	// Update tip hash
	hash2, err := dao.getBlockHash(height - 1)
	if err != nil {
		return errors.Wrap(err, "failed to get tip block hash")
	}
	batch.Put(blockNS, topHashKey, hash2[:], "failed to put top hash")

	return dao.kvstore.Commit(batch)
}

func (dao *blockDAO) getTopDB(blkHeight uint64) (kvstore db.KVStore, index uint64, err error) {
	topIndex, ok := dao.topIndex.Load().(uint64)
	if !ok {
		topIndex = 0
	}
	file, dir := getFileNameAndDir(dao.cfg.DbPath)
	if err != nil {
		return
	}
	longFileName := dir + "/" + file + fmt.Sprintf("-%08d", topIndex) + ".db"
	dat, err := os.Stat(longFileName)
	if err != nil && os.IsNotExist(err) {
		// index the height --> file index mapping
		if err = dao.IndexFile(blkHeight, byteutil.Uint64ToBytesBigEndian(topIndex)); err != nil {
			return
		}
		// db file does not exist, create it
		return dao.openDB(topIndex)
	}
	// other errors except file does not exist
	if err != nil {
		return
	}
	// file exists,but need create new db
	if dao.cfg.SplitDBSizeMB > 0 && uint64(dat.Size()) > dao.cfg.SplitDBSize() {
		kvstore, index, err = dao.openDB(topIndex + 1)
		dao.topIndex.Store(index)
		// index the height --> file index mapping
		err = dao.IndexFile(blkHeight, byteutil.Uint64ToBytesBigEndian(index))
		return
	}
	// db exist,need load from kvstores
	kv, ok := dao.kvstores.Load(topIndex)
	if ok {
		kvstore, ok = kv.(db.KVStore)
		if !ok {
			err = errors.New("db convert error")
		}
		index = topIndex
		return
	}
	// file exists,but not opened
	return dao.openDB(topIndex)
}

func (dao *blockDAO) getDBFromHeight(blkHeight uint64) (kvstore db.KVStore, index uint64, err error) {
	// get file index
	value, err := dao.GetFileIndex(blkHeight)
	if err != nil {
		return
	}
	return dao.getDBFromIndex(byteutil.BytesToUint64BigEndian(value))
}

func (dao *blockDAO) getDBFromIndex(idx uint64) (kvstore db.KVStore, index uint64, err error) {
	kv, ok := dao.kvstores.Load(idx)
	if ok {
		kvstore, ok = kv.(db.KVStore)
		if !ok {
			err = errors.New("db convert error")
		}
		index = idx
		return
	}
	// if user rm some db files manully,then call this method will create new file
	return dao.openDB(idx)
}

// openDB open file if exists, or create new file
func (dao *blockDAO) openDB(idx uint64) (kvstore db.KVStore, index uint64, err error) {
	dao.mutex.Lock()
	defer dao.mutex.Unlock()
	cfg := dao.cfg
	model, _ := getFileNameAndDir(cfg.DbPath)
	name := model + fmt.Sprintf("-%08d", idx) + ".db"

	// open or create this db file
	cfg.DbPath = path.Dir(cfg.DbPath) + "/" + name
	kvstore = db.NewBoltDB(cfg)
	dao.kvstores.Store(idx, kvstore)
	err = kvstore.Start(context.Background())
	if err != nil {
		return
	}
	dao.lifecycle.Add(kvstore)
	index = idx
	return
}

func getFileNameAndDir(p string) (fileName, dir string) {
	var withSuffix, suffix string
	withSuffix = path.Base(p)
	suffix = path.Ext(withSuffix)
	fileName = strings.TrimSuffix(withSuffix, suffix)
	dir = path.Dir(p)
	return
}

func hashKey(h hash.Hash256) []byte {
	return append(hashPrefix, h[:]...)
}

func heightKey(height uint64) []byte {
	return append(heightPrefix, byteutil.Uint64ToBytes(height)...)
}
