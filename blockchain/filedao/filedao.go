// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package filedao

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"go.uber.org/zap"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/blockchain/block"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

const (
	blockHashHeightMappingNS = "h2h"
	systemLogNS              = "syl"
)

var (
	topHeightKey = []byte("th")
	topHashKey   = []byte("ts")
	hashPrefix   = []byte("ha.")
)

// vars
var (
	ErrFileNotExist     = errors.New("file does not exist")
	ErrFileInvalid      = errors.New("file format is not valid")
	ErrNotSupported     = errors.New("feature not supported")
	ErrAlreadyExist     = errors.New("block already exist")
	ErrInvalidTipHeight = errors.New("invalid tip height")
	ErrDataCorruption   = errors.New("data is corrupted")
)

type (
	// FileDAO represents the data access object for managing block db file
	FileDAO interface {
		Start(ctx context.Context) error
		Stop(ctx context.Context) error
		Height() (uint64, error)
		GetBlockHash(uint64) (hash.Hash256, error)
		GetBlockHeight(hash.Hash256) (uint64, error)
		GetBlock(hash.Hash256) (*block.Block, error)
		GetBlockByHeight(uint64) (*block.Block, error)
		GetReceipts(uint64) ([]*action.Receipt, error)
		ContainsTransactionLog() bool
		TransactionLogs(uint64) (*iotextypes.TransactionLogs, error)
		PutBlock(context.Context, *block.Block) error
		DeleteTipBlock() error
	}

	// fileDAO implements FileDAO
	fileDAO struct {
		currFd   FileDAO
		legacyFd FileDAO
		v2Fd     FileV2Manager // a collection of v2 db files
	}
)

// NewFileDAO creates an instance of FileDAO
func NewFileDAO(cfg config.DB) (FileDAO, error) {
	header, v2Files, err := checkChainDBFiles(cfg)
	if err == ErrFileInvalid {
		return nil, err
	}

	if err == ErrFileNotExist {
		// start new chain db using v2 format
		if err := createNewV2File(1, cfg); err != nil {
			return nil, err
		}
		return CreateFileDAO(false, []string{cfg.DbPath}, cfg)
	}

	switch header.Version {
	case FileLegacyMaster:
		// default file is legacy format
		return CreateFileDAO(true, v2Files, cfg)
	case FileV2:
		// default file is v2 format, add it to filenames
		v2Files = append(v2Files, cfg.DbPath)
		return CreateFileDAO(false, v2Files, cfg)
	default:
		panic(fmt.Errorf("corrupted file version: %s", header.Version))
	}
}

// NewFileDAOInMemForTest creates an in-memory FileDAO for testing
func NewFileDAOInMemForTest(cfg config.DB) (FileDAO, error) {
	legacyFd, err := newFileDAOLegacyInMem(cfg.CompressLegacy, cfg)
	if err != nil {
		return nil, err
	}

	return &fileDAO{legacyFd: legacyFd}, nil
}

func (fd *fileDAO) Start(ctx context.Context) error {
	if fd.legacyFd != nil {
		if err := fd.legacyFd.Start(ctx); err != nil {
			return err
		}
	}
	if fd.v2Fd != nil {
		if err := fd.v2Fd.Start(ctx); err != nil {
			return err
		}
	}

	if fd.v2Fd != nil {
		fd.currFd = fd.v2Fd.TopFd()
	} else {
		fd.currFd = fd.legacyFd
	}
	return nil
}

func (fd *fileDAO) Stop(ctx context.Context) error {
	if fd.legacyFd != nil {
		if err := fd.legacyFd.Stop(ctx); err != nil {
			return err
		}
	}
	if fd.v2Fd != nil {
		return fd.v2Fd.Stop(ctx)
	}
	return nil
}

func (fd *fileDAO) Height() (uint64, error) {
	return fd.currFd.Height()
}

func (fd *fileDAO) GetBlockHash(height uint64) (hash.Hash256, error) {
	if fd.v2Fd != nil {
		if height == 0 {
			return hash.ZeroHash256, nil
		}
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.GetBlockHash(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockHash(height)
	}
	return hash.ZeroHash256, ErrNotSupported
}

func (fd *fileDAO) GetBlockHeight(hash hash.Hash256) (uint64, error) {
	var (
		height uint64
		err    error
	)
	for _, file := range fd.v2Fd {
		if height, err = file.fd.GetBlockHeight(hash); err == nil {
			return height, nil
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockHeight(hash)
	}
	return 0, err
}

func (fd *fileDAO) GetBlock(hash hash.Hash256) (*block.Block, error) {
	var (
		blk *block.Block
		err error
	)
	for _, file := range fd.v2Fd {
		if blk, err = file.fd.GetBlock(hash); err == nil {
			return blk, nil
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlock(hash)
	}
	return nil, err
}

func (fd *fileDAO) GetBlockByHeight(height uint64) (*block.Block, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.GetBlockByHeight(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetBlockByHeight(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) GetReceipts(height uint64) ([]*action.Receipt, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.GetReceipts(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.GetReceipts(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) ContainsTransactionLog() bool {
	// TODO: change to ContainsTransactionLog(uint64)
	return fd.currFd.ContainsTransactionLog()
}

func (fd *fileDAO) TransactionLogs(height uint64) (*iotextypes.TransactionLogs, error) {
	if fd.v2Fd != nil {
		if v2 := fd.v2Fd.FileDAOByHeight(height); v2 != nil {
			return v2.TransactionLogs(height)
		}
	}

	if fd.legacyFd != nil {
		return fd.legacyFd.TransactionLogs(height)
	}
	return nil, ErrNotSupported
}

func (fd *fileDAO) PutBlock(ctx context.Context, blk *block.Block) error {
	// bail out if block already exists
	h := blk.HashBlock()
	if _, err := fd.GetBlockHeight(h); err == nil {
		log.L().Error("Block already exists.", zap.Uint64("height", blk.Height()), log.Hex("hash", h[:]))
		return ErrAlreadyExist
	}
	// TODO: check if need to split DB
	return fd.currFd.PutBlock(ctx, blk)
}

func (fd *fileDAO) DeleteTipBlock() error {
	return fd.currFd.DeleteTipBlock()
}

// CreateFileDAO creates FileDAO from legacy and new files
func CreateFileDAO(legacy bool, v2Files []string, cfg config.DB) (FileDAO, error) {
	if legacy == false && len(v2Files) == 0 {
		return nil, ErrNotSupported
	}

	var (
		legacyFd FileDAO
		err      error
	)
	if legacy {
		legacyFd, err = newFileDAOLegacy(cfg)
		if err != nil {
			return nil, err
		}
	}

	var v2Fd FileV2Manager
	if len(v2Files) > 0 {
		fds := make([]*fileDAOv2, len(v2Files))
		for i, name := range v2Files {
			cfg.DbPath = name
			fds[i] = openFileDAOv2(cfg)
		}
		v2Fd = NewFileV2Manager(fds)
	}

	return &fileDAO{
		legacyFd: legacyFd,
		v2Fd:     v2Fd,
	}, nil
}

// createNewV2File creates a new v2 chain db file
func createNewV2File(start uint64, cfg config.DB) error {
	v2, err := newFileDAOv2(start, cfg)
	if err != nil {
		return err
	}

	// calling Start() will write the header
	ctx := context.Background()
	if err := v2.Start(ctx); err != nil {
		return err
	}
	return v2.Stop(ctx)
}
