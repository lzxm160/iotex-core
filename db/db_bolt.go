// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"strings"

	"github.com/iotexproject/iotex-core/state"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/log"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const fileMode = 0600

// ContractKVNameSpace for ignore delete
var ContractKVNameSpace = "Contract"

// boltDB is KVStore implementation based bolt DB
type boltDB struct {
	db     *bolt.DB
	path   string
	config config.DB
}

// NewBoltDB instantiates an BoltDB with implements KVStore
func NewBoltDB(cfg config.DB) KVStore {
	return &boltDB{db: nil, path: cfg.DbPath, config: cfg}
}

// Start opens the BoltDB (creates new file if not existing yet)
func (b *boltDB) Start(_ context.Context) error {
	db, err := bolt.Open(b.path, fileMode, nil)
	if err != nil {
		return errors.Wrap(ErrIO, err.Error())
	}
	b.db = db
	return nil
}

// Stop closes the BoltDB
func (b *boltDB) Stop(_ context.Context) error {
	if b.db != nil {
		if err := b.db.Close(); err != nil {
			return errors.Wrap(ErrIO, err.Error())
		}
	}
	return nil
}

// Put inserts a <key, value> record
func (b *boltDB) Put(namespace string, key, value []byte) (err error) {
	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
			if err != nil {
				return err
			}
			return bucket.Put(key, value)
		}); err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// Get retrieves a record
func (b *boltDB) Get(namespace string, key []byte) ([]byte, error) {
	var value []byte
	err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(namespace))
		if bucket == nil {
			return errors.Wrapf(ErrNotExist, "bucket = %s doesn't exist", namespace)
		}
		v := bucket.Get(key)
		if v == nil {
			return errors.Wrapf(ErrNotExist, "key = %x doesn't exist", key)
		}
		value = make([]byte, len(v))
		// TODO: this is not an efficient way of passing the data
		copy(value, v)
		return nil
	})
	if err == nil {
		return value, nil
	}
	if errors.Cause(err) == ErrNotExist {
		return nil, err
	}
	return nil, errors.Wrap(ErrIO, err.Error())
}

// GetPrefix retrieves all keys those with const prefix
func (b *boltDB) GetPrefix(namespace string, prefix []byte) ([][]byte, error) {
	allKey := make([][]byte, 0)
	err := b.db.View(func(tx *bolt.Tx) error {
		buck := tx.Bucket([]byte(namespace))
		if buck == nil {
			return ErrNotExist
		}
		c := buck.Cursor()

		for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
			allKey = append(allKey, k)
		}
		return nil
	})
	if err == nil {
		return allKey, nil
	}
	if errors.Cause(err) == ErrNotExist {
		return nil, err
	}
	return nil, errors.Wrap(ErrIO, err.Error())
}

// GetPrefixRange return the first value which key < maxKey
func (b *boltDB) GetPrefixRange(namespace string, minKey []byte, maxKey []byte, targetHeight uint64, s interface{}) error {
	return b.db.View(func(tx *bolt.Tx) error {
		buck := tx.Bucket([]byte(namespace))
		if buck == nil {
			return ErrNotExist
		}
		c := buck.Cursor()
		for k, v := c.Seek(maxKey); k != nil && bytes.Compare(k, minKey) >= 0; k, v = c.Prev() {
			if len(k) <= 20 {
				return ErrNotExist
			}
			kHeight := binary.BigEndian.Uint64(k[20:])
			log.L().Info("////////////////", zap.Uint64("k", kHeight), zap.Uint64("height", targetHeight))
			if kHeight == 0 || kHeight == 1 {
				return ErrNotExist
			}
			if kHeight <= targetHeight {
				log.L().Info("////////////////", zap.Uint64("k", kHeight), zap.Uint64("height", targetHeight))
				if err := state.Deserialize(s, v); err != nil {
					return errors.Wrapf(err, "error when deserializing state data into %T", s)
				}
				return nil
			}
		}
		return ErrNotExist
	})
}

// Delete deletes a record,if key is nil,this will delete the whole bucket
func (b *boltDB) Delete(namespace string, key []byte) (err error) {
	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		if key == nil {
			err = b.db.Update(func(tx *bolt.Tx) error {
				if err := tx.DeleteBucket([]byte(namespace)); err != bolt.ErrBucketNotFound {
					return err
				}
				return nil
			})
		} else {
			err = b.db.Update(func(tx *bolt.Tx) error {
				bucket := tx.Bucket([]byte(namespace))
				if bucket == nil {
					return nil
				}
				return bucket.Delete(key)
			})
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// Commit commits a batch
func (b *boltDB) Commit(batch KVStoreBatch) (err error) {
	succeed := true
	batch.Lock()
	defer func() {
		if succeed {
			// clear the batch if commit succeeds
			batch.ClearAndUnlock()
		} else {
			batch.Unlock()
		}

	}()

	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		if err = b.db.Update(func(tx *bolt.Tx) error {
			for i := 0; i < batch.Size(); i++ {
				write, err := batch.Entry(i)
				if err != nil {
					return err
				}
				if write.writeType == Put {
					if write.namespace == ContractKVNameSpace {
						log.L().Info("len of ContractKVNameSpace commit", zap.Int("trie batch size ", batch.Size()), zap.String("save key", hex.EncodeToString(write.key)))
					}
					bucket, err := tx.CreateBucketIfNotExists([]byte(write.namespace))
					if err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
					if err := bucket.Put(write.key, write.value); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				} else if write.writeType == Delete {
					if !strings.EqualFold(write.namespace, ContractKVNameSpace) {
						bucket := tx.Bucket([]byte(write.namespace))
						if bucket == nil {
							continue
						}
						if err := bucket.Delete(write.key); err != nil {
							return errors.Wrapf(err, write.errorFormat, write.errorArgs)
						}
					}
				}
			}
			return nil
		}); err == nil {
			break
		}
	}

	if err != nil {
		succeed = false
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

// CountingIndex returns the index, and nil if not exist
func (b *boltDB) CountingIndex(name []byte) (CountingIndex, error) {
	var total []byte
	if err := b.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(name)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", name)
		}
		// get the number of keys
		total = bucket.Get(ZeroIndex)
		if total == nil {
			return errors.Wrap(ErrNotExist, "total count doesn't exist")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return NewCountingIndex(b.db, b.config.NumRetries, name, byteutil.BytesToUint64BigEndian(total))
}

// CreateCountingIndexNX creates a new index if it does not exist, otherwise return existing index
func (b *boltDB) CreateCountingIndexNX(name []byte) (CountingIndex, error) {
	var size uint64
	if err := b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists(name)
		if err != nil {
			return errors.Wrapf(err, "failed to create bucket %x", name)
		}
		// check the number of keys
		total := bucket.Get(ZeroIndex)
		if total == nil {
			// put 0 as total number of keys
			return bucket.Put(ZeroIndex, ZeroIndex)
		}
		size = byteutil.BytesToUint64BigEndian(total)
		return nil
	}); err != nil {
		return nil, err
	}
	return NewCountingIndex(b.db, b.config.NumRetries, name, size)
}

//======================================
// private functions
//======================================

// intentionally fail to test DB can successfully rollback
func (b *boltDB) batchPutForceFail(namespace string, key [][]byte, value [][]byte) error {
	return b.db.Update(func(tx *bolt.Tx) error {
		bucket, err := tx.CreateBucketIfNotExists([]byte(namespace))
		if err != nil {
			return err
		}
		if len(key) != len(value) {
			return errors.Wrap(ErrIO, "batch put <k, v> size not match")
		}
		for i := 0; i < len(key); i++ {
			if err := bucket.Put(key[i], value[i]); err != nil {
				return err
			}
			// intentionally fail to test DB can successfully rollback
			if i == len(key)-1 {
				return errors.Wrapf(ErrIO, "force fail to test DB rollback")
			}
		}
		return nil
	})
}
