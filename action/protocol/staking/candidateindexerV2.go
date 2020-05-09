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

var (
	// CandidateV2Namespace is a namespace to store candidates with epoch start height
	CandidateV2Namespace = "candidatesV2"
)

// CandidateV2Indexer is an indexer to store candidates by given height
type CandidateV2Indexer struct {
	mutex   sync.RWMutex
	kvStore db.KVStore
}

// NewCandidateV2Indexer creates a new CandidateV2Indexer
func NewCandidateV2Indexer(kv db.KVStore) (*CandidateV2Indexer, error) {
	if kv == nil {
		return nil, errors.New("empty kvStore")
	}
	x := CandidateV2Indexer{
		kvStore: kv,
	}
	return &x, nil
}

// Start starts the indexer
func (vb *CandidateV2Indexer) Start(ctx context.Context) error {
	return vb.kvStore.Start(ctx)
}

// Stop stops the indexer
func (vb *CandidateV2Indexer) Stop(ctx context.Context) error {
	return vb.kvStore.Stop(ctx)
}

// Put puts vote buckets into indexer
func (vb *CandidateV2Indexer) Put(height uint64, candidates *iotextypes.CandidateListV2) error {
	vb.mutex.Lock()
	defer vb.mutex.Unlock()
	candidatesBytes, err := proto.Marshal(candidates)
	if err != nil {
		return err
	}
	for _, cand := range candidates.Candidates {
		fmt.Println("CandidateV2Indexer Put", height, cand)
	}

	return vb.kvStore.Put(CandidateV2Namespace, byteutil.Uint64ToBytes(height), candidatesBytes)
}

// Get gets vote buckets from indexer given epoch start height
func (vb *CandidateV2Indexer) Get(height uint64) ([]byte, error) {
	vb.mutex.RLock()
	defer vb.mutex.RUnlock()

	ret, err := vb.kvStore.Get(CandidateV2Namespace, byteutil.Uint64ToBytes(height))
	if errors.Cause(err) == db.ErrNotExist {
		return proto.Marshal(&iotextypes.CandidateListV2{})
	}
	return ret, err
}