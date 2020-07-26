// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package staking

import (
	"context"
	"fmt"
	"sync"

	"github.com/gogo/protobuf/proto"
	"github.com/pkg/errors"

	"github.com/iotexproject/iotex-proto/golang/iotextypes"

	"github.com/iotexproject/iotex-core/db"
	"github.com/iotexproject/iotex-core/pkg/util/byteutil"
)

const (
	// StakingCandidatesNamespace is a namespace to store candidates with epoch start height
	StakingCandidatesNamespace = "stakingCandidates"
	// StakingBucketsNamespace is a namespace to store vote buckets with epoch start height
	StakingBucketsNamespace = "stakingBuckets"
)

// CandidatesBucketsIndexer is an indexer to store candidates by given height
type CandidatesBucketsIndexer struct {
	mutex   sync.RWMutex
	kvStore db.KVStore
}

// NewStakingCandidatesBucketsIndexer creates a new StakingCandidatesIndexer
func NewStakingCandidatesBucketsIndexer(kv db.KVStore) (*CandidatesBucketsIndexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	return &CandidatesBucketsIndexer{
		kvStore: kv,
	}, nil
}

// Start starts the indexer
func (cbi *CandidatesBucketsIndexer) Start(ctx context.Context) error {
	return cbi.kvStore.Start(ctx)
}

// Stop stops the indexer
func (cbi *CandidatesBucketsIndexer) Stop(ctx context.Context) error {
	return cbi.kvStore.Stop(ctx)
}

// PutCandidates puts candidates into indexer
func (cbi *CandidatesBucketsIndexer) PutCandidates(height uint64, candidates *iotextypes.CandidateListV2) error {
	fmt.Println("PutCandidates", len(candidates.Candidates))
	cbi.mutex.Lock()
	defer cbi.mutex.Unlock()
	candidatesBytes, err := proto.Marshal(candidates)
	if err != nil {
		return err
	}
	return cbi.kvStore.Put(StakingCandidatesNamespace, byteutil.Uint64ToBytes(height), candidatesBytes)
}

// GetCandidates gets candidates from indexer given epoch start height
func (cbi *CandidatesBucketsIndexer) GetCandidates(height uint64, offset, limit uint32) ([]byte, error) {
	cbi.mutex.RLock()
	defer cbi.mutex.RUnlock()
	candidateList := &iotextypes.CandidateListV2{}
	ret, err := cbi.kvStore.Get(StakingCandidatesNamespace, byteutil.Uint64ToBytes(height))
	if errors.Cause(err) == db.ErrNotExist {
		return proto.Marshal(candidateList)
	}
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(ret, candidateList); err != nil {
		return nil, err
	}
	length := uint32(len(candidateList.Candidates))
	if offset >= length {
		return proto.Marshal(&iotextypes.CandidateListV2{})
	}
	end := offset + limit
	if end > uint32(len(candidateList.Candidates)) {
		end = uint32(len(candidateList.Candidates))
	}
	candidateList.Candidates = candidateList.Candidates[offset:end]
	return proto.Marshal(candidateList)
}

// PutBuckets puts vote buckets into indexer
func (cbi *CandidatesBucketsIndexer) PutBuckets(height uint64, buckets *iotextypes.VoteBucketList) error {
	cbi.mutex.Lock()
	defer cbi.mutex.Unlock()
	fmt.Println("PutBuckets", len(buckets.Buckets))
	bucketsBytes, err := proto.Marshal(buckets)
	if err != nil {
		return err
	}
	return cbi.kvStore.Put(StakingBucketsNamespace, byteutil.Uint64ToBytes(height), bucketsBytes)
}

// GetBuckets gets vote buckets from indexer given epoch start height
func (cbi *CandidatesBucketsIndexer) GetBuckets(height uint64, offset, limit uint32) ([]byte, error) {
	cbi.mutex.RLock()
	defer cbi.mutex.RUnlock()
	buckets := &iotextypes.VoteBucketList{}
	ret, err := cbi.kvStore.Get(StakingBucketsNamespace, byteutil.Uint64ToBytes(height))
	if errors.Cause(err) == db.ErrNotExist {
		return proto.Marshal(buckets)
	}
	if err != nil {
		return nil, err
	}
	if err := proto.Unmarshal(ret, buckets); err != nil {
		return nil, err
	}
	length := uint32(len(buckets.Buckets))
	if offset >= length {
		return proto.Marshal(&iotextypes.VoteBucketList{})
	}
	end := offset + limit
	if end > uint32(len(buckets.Buckets)) {
		end = uint32(len(buckets.Buckets))
	}
	buckets.Buckets = buckets.Buckets[offset:end]
	return proto.Marshal(buckets)
}
