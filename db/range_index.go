// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"

	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"

	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

var (
	// CurrIndex is special key such that bytes.Compare(MaxUint64, CurrIndex) = -1
	MaxKey = []byte{255, 255, 255, 255, 255, 255, 255, 255, 0}
	//InitValue     =
	NotExist      = []byte("InitValue")
	NotExistValue = []byte("NotExistValue")
)

type (
	// RangeIndex is a bucket of sparse <k, v> pair, where k consists of 8-byte value
	// and all keys that falls in 2 consecutive k have the same v
	// for example, given 3 entries in the bucket:
	//
	// k = 0x0000000000000004 ==> v1
	// k = 0x0000000000000123 ==> v2
	// k = 0x0000000000005678 ==> v3
	//
	// we have:
	// for all key   0x0 <= k <  0x4,    value[k] = initial value
	// for all key   0x4 <= k <  0x123,  value[k] = v1
	// for all key 0x123 <= k <  0x5678, value[k] = v2
	// for all key          k >= 0x5678, value[k] = v3
	//
	// position 0 (k = 0x0000000000000000) stores the value to return beyond the largest inserted key
	//
	RangeIndex interface {
		// Insert inserts a value into the index
		Insert(uint64, []byte) error
		// Get returns value by the key
		Get(uint64) ([]byte, error)
		// Close makes the index not usable
		Close()
		// Delete deletes key before this key but keep this key
		Delete(key uint64) error
		Purge(key uint64) error
	}

	// rangeIndex is RangeIndex implementation based on bolt DB
	rangeIndex struct {
		db         *bolt.DB
		numRetries uint8
		bucket     []byte
		curr       []byte // value to return beyond the last insertion key
	}
)

// NewRangeIndex creates a new instance of rangeIndex
func NewRangeIndex(db *bolt.DB, retry uint8, name []byte) (RangeIndex, error) {
	if db == nil {
		return nil, errors.Wrap(ErrInvalid, "db object is nil")
	}

	if len(name) == 0 {
		return nil, errors.Wrap(ErrInvalid, "bucket name is nil")
	}

	bucket := make([]byte, len(name))
	copy(bucket, name)
	var curr []byte
	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucket)
		if b == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", bucket)
		}
		// check whether init value exist or not
		curr = b.Get(MaxKey)
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "bucket get CurrIndex error")
	}
	return &rangeIndex{
		db:         db,
		numRetries: retry,
		bucket:     bucket,
		curr:       curr,
	}, nil
}

// Insert inserts a value into the index
func (r *rangeIndex) Insert(key uint64, value []byte) error {
	defer r.Close()
	// cannot insert key 0, which holds key-1's value
	if key == 0 {
		return errors.Wrap(ErrInvalid, "cannot insert key 0")
	}
	var err error
	for i := uint8(0); i < r.numRetries; i++ {
		if err = r.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(r.bucket)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", r.bucket)
			}
			// read current value
			curr := bucket.Get(MaxKey)
			if curr == nil {
				return errors.Wrap(ErrIO, "cannot read current value")
			}
			// check if key already exists, to fix putstate insert 2 times for the same key
			keySub1 := bucket.Get(byteutil.Uint64ToBytesBigEndian(key - 1))
			if keySub1 == nil {
				// keys up to key-1 should have current value
				if err := bucket.Put(byteutil.Uint64ToBytesBigEndian(key-1), curr); err != nil {
					return err
				}
			}
			//else {
			//	log.L().Info("keySub1 already exists", zap.Uint64("height", key-1))
			//}
			// write new value
			return bucket.Put(MaxKey, value)
		}); err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	r.curr = nil
	r.curr = make([]byte, len(value))
	copy(r.curr, value)
	return nil
}

// Get returns value by the key
func (r *rangeIndex) Get(key uint64) ([]byte, error) {
	defer r.Close()
	var value []byte
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(r.bucket)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", r.bucket)
		}
		// seek to start
		cur := bucket.Cursor()
		k, v := cur.Seek(byteutil.Uint64ToBytesBigEndian(key))
		if k == nil {
			// key is beyond largest inserted key, return current value
			v = r.curr
		}

		if bytes.Compare(v, NotExistValue) == 0 {
			return errors.New("key already deleted")
		}
		if bytes.Compare(v, InitValue) == 0 {
			return errors.New("key not exist")
		}
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Delete deletes key before this key include this key
func (r *rangeIndex) Delete(key uint64) error {
	defer r.Close()
	// cannot delete key 0, which holds key-1's value
	if key == 0 {
		return errors.Wrap(ErrInvalid, "cannot delete key 0")
	}
	err := r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(r.bucket)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", r.bucket)
		}
		// seek to start
		cur := bucket.Cursor()
		// find the key and set to special value
		for k, v := cur.Seek(byteutil.Uint64ToBytesBigEndian(key)); k != nil && bytes.Compare(v, NotExistValue) != 0; k, v = cur.Prev() {
			if k == nil {
				break
			}
			log.L().Info("rangeIndex Delete/////////", zap.Uint64("height", byteutil.BytesToUint64BigEndian(k)))
			if err := bucket.Put(k, NotExistValue); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}
func (r *rangeIndex) Purge(key uint64) error {
	defer r.Close()
	// cannot delete key 0, which holds key-1's value
	if key == 0 {
		return errors.Wrap(ErrInvalid, "cannot delete key 0")
	}
	err := r.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(r.bucket)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", r.bucket)
		}
		// seek to start
		cur := bucket.Cursor()
		// find the key and set to special value
		for k, v := cur.Seek(byteutil.Uint64ToBytesBigEndian(key)); k != nil && bytes.Compare(v, NotExistValue) != 0; k, v = cur.Prev() {
			if k == nil {
				break
			}
			log.L().Info("rangeIndex Delete/////////", zap.Uint64("height", byteutil.BytesToUint64BigEndian(k)))
			if err := bucket.Put(k, NotExistValue); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

// Close makes the index not usable
func (r *rangeIndex) Close() {
	// frees reference to db, the db object itself will be closed/freed by its owner, not here
	r.db = nil
	r.bucket = nil
}
