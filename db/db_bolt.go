// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/config"
)

const fileMode = 0600

// boltDB is KVStore implementation based bolt DB
type boltDB struct {
	db     map[int]*bolt.DB
	path   string
	config config.DB
}

// Start opens the BoltDB (creates new file if not existing yet)
func (b *boltDB) Start(_ context.Context) error {
	db, err := bolt.Open(b.path+"-0", fileMode, nil)
	if err != nil {
		return errors.Wrap(ErrIO, err.Error())
	}
	b.db[0] = db
	return nil
}

// Stop closes the BoltDB
func (b *boltDB) Stop(_ context.Context) error {
	for _, v := range b.db {
		if v != nil {
			if err := v.Close(); err != nil {
				return errors.Wrap(ErrIO, err.Error())
			}
		}
	}

	return nil
}

// Put inserts a <key, value> record
func (b *boltDB) Put(namespace string, key, value []byte, whichDB int) (err error) {
	db, ok := b.db[whichDB]
	if !ok {
		db, err = bolt.Open(b.path+"-"+fmt.Sprintf("%d", whichDB), fileMode, nil)
		if err != nil {
			return errors.Wrap(ErrIO, err.Error())
		}
		b.db[whichDB] = db
	}
	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		if err = db.Update(func(tx *bolt.Tx) error {
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
func (b *boltDB) Get(namespace string, key []byte, whichDB int) ([]byte, error) {
	db, ok := b.db[whichDB]
	if !ok {
		return nil, errors.New("db is not exist")
	}
	var value []byte
	err := db.View(func(tx *bolt.Tx) error {
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

// Delete deletes a record
func (b *boltDB) Delete(namespace string, key []byte, whichDB int) (err error) {
	db, ok := b.db[whichDB]
	if !ok {
		return errors.New("db is not exist")
	}
	numRetries := b.config.NumRetries
	for c := uint8(0); c < numRetries; c++ {
		err = db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(namespace))
			if bucket == nil {
				return nil
			}
			return bucket.Delete(key)
		})
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
	//db, ok := b.db[whichDB]
	//if !ok {
	//	db, err = bolt.Open(b.path+"-"+fmt.Sprintf("%d", whichDB), fileMode, nil)
	//	if err != nil {
	//		return errors.Wrap(ErrIO, err.Error())
	//	}
	//	b.db[whichDB] = db
	//}
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
	for i := 0; i < batch.Size(); i++ {
		for c := uint8(0); c < numRetries; c++ {
			if err = db.Update(func(tx *bolt.Tx) error {
				write, err := batch.Entry(i)
				if err != nil {
					return err
				}
				if write.writeType == Put {
					bucket, err := tx.CreateBucketIfNotExists([]byte(write.namespace))
					if err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
					if err := bucket.Put(write.key, write.value); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				} else if write.writeType == Delete {
					bucket := tx.Bucket([]byte(write.namespace))
					if bucket == nil {
						continue
					}
					if err := bucket.Delete(write.key); err != nil {
						return errors.Wrapf(err, write.errorFormat, write.errorArgs)
					}
				}

				return nil
			}); err == nil {
				break
			}
		}
	}

	if err != nil {
		succeed = false
		err = errors.Wrap(ErrIO, err.Error())
	}
	return err
}

//======================================
// private functions
//======================================

// intentionally fail to test DB can successfully rollback
func (b *boltDB) batchPutForceFail(namespace string, key [][]byte, value [][]byte, whichDB int) error {
	db, ok := b.db[whichDB]
	if !ok {
		return errors.New("db is not exist")
	}
	return db.Update(func(tx *bolt.Tx) error {
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
