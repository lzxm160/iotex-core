// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"bytes"

	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
	"github.com/pkg/errors"
	bolt "go.etcd.io/bbolt"
)

var (
// MaxKey is the special key such that bytes.Compare(MaxUint64, MaxKey) = -1
//MaxKey = []byte{255, 255, 255, 255, 255, 255, 255, 255, 0}
// NotExist is the empty byte slice to indicate a key does not exist (as a result of calling Purge())
//NotExist = []byte{}
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
	//RangeIndex interface {
	//	// Insert inserts a value into the index
	//	Insert(uint64, []byte) error
	//	// Get returns value by the key
	//	Get(uint64) ([]byte, error)
	//	// Delete deletes an existing key
	//	Delete(uint64) error
	//	// Purge deletes an existing key and all keys before it
	//	Purge(uint64) error
	//	// Close makes the index not usable
	//	Close()
	//}

	// rangeIndex is RangeIndex implementation based on bolt DB
	rangeIndexForHistory struct {
		db         *bolt.DB
		numRetries uint8
		bucket     []byte
	}
)

// NewRangeIndex creates a new instance of rangeIndex
func NewRangeIndexForHistory(db *bolt.DB, retry uint8, name []byte) (RangeIndex, error) {
	if db == nil {
		return nil, errors.Wrap(ErrInvalid, "db object is nil")
	}

	if len(name) == 0 {
		return nil, errors.Wrap(ErrInvalid, "bucket name is nil")
	}

	bucket := make([]byte, len(name))
	copy(bucket, name)

	return &rangeIndexForHistory{
		db:         db,
		numRetries: retry,
		bucket:     bucket,
	}, nil
}

// Insert inserts a value into the index
func (r *rangeIndexForHistory) Insert(key uint64, value []byte) error {
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
			cur := bucket.Cursor()
			ak := byteutil.Uint64ToBytesBigEndian(key - 1)
			k, v := cur.Seek(ak)
			if !bytes.Equal(k, ak) {
				// insert new key
				bucket.Put(ak, v)
			} else {
				// update an existing key
				k, _ = cur.Next()
			}
			return bucket.Put(k, value)
		}); err == nil {
			break
		}
	}
	if err != nil {
		err = errors.Wrap(ErrIO, err.Error())
	}
	return nil
}

// Get returns value by the key
func (r *rangeIndexForHistory) Get(key uint64) ([]byte, error) {
	var value []byte
	err := r.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket(r.bucket)
		if bucket == nil {
			return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", r.bucket)
		}
		// seek to start
		cur := bucket.Cursor()
		_, v := cur.Seek(byteutil.Uint64ToBytesBigEndian(key))
		value = make([]byte, len(v))
		copy(value, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return value, nil
}

// Delete deletes an existing key
func (r *rangeIndexForHistory) Delete(key uint64) error {
	// cannot delete key 0, which holds initial value
	if key == 0 {
		return errors.Wrap(ErrInvalid, "cannot delete key 0")
	}
	var err error
	for i := uint8(0); i < r.numRetries; i++ {
		if err = r.db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket(r.bucket)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", r.bucket)
			}
			cur := bucket.Cursor()
			ak := byteutil.Uint64ToBytesBigEndian(key - 1)
			k, v := cur.Seek(ak)
			if !bytes.Equal(k, ak) {
				// return nil if the key does not exist
				return nil
			}
			bucket.Delete(ak)
			// write the corresponding value to next key
			k, _ = cur.Seek(byteutil.Uint64ToBytesBigEndian(key))
			return bucket.Put(k, v)
		}); err == nil {
			break
		}
	}
	return err
}

// Purge deletes an existing key and all keys before it
func (r *rangeIndexForHistory) Purge(key uint64) error {
	defer r.Close()
	// cannot delete key 0, which holds initial value
	if key == 0 {
		return errors.Wrap(ErrInvalid, "cannot delete key 0")
	}
	var err error
	for i := uint8(0); i < r.numRetries; i++ {
		if err = r.db.Update(func(tx *bolt.Tx) error {

			bucket := tx.Bucket(r.bucket)
			if bucket == nil {
				return errors.Wrapf(ErrBucketNotExist, "bucket = %x doesn't exist", r.bucket)
			}
			cur := bucket.Cursor()
			nextk := byteutil.Uint64ToBytesBigEndian(key + 1)
			nextK, nextV := cur.Seek(nextk)
			ak := byteutil.Uint64ToBytesBigEndian(key - 1)
			k, _ := cur.Seek(ak)
			if !bytes.Equal(k, ak) {
				// return nil if the key does not exist
				return nil
			}
			// delete all keys before this key
			for ; k != nil; k, _ = cur.Prev() {
				bucket.Delete(k)
			}
			// write not exist value to next key
			k, _ = cur.Seek(byteutil.Uint64ToBytesBigEndian(key))
			err = bucket.Put(k, NotExist)
			if err != nil {
				return err
			}
			if !bytes.Equal(nextV, NotExist) {
				return r.Insert(byteutil.BytesToUint64BigEndian(nextK), nextV)
			}
			return err
		}); err == nil {
			break
		}
	}
	return err
}

// Close makes the index not usable
func (r *rangeIndexForHistory) Close() {
	// frees reference to db, the db object itself will be closed/freed by its owner, not here
	r.db = nil
	r.bucket = nil
}
