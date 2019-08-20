// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-core/action/protocol/execution/evm"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/db/trie"
	"github.com/iotexproject/iotex-core/pkg/log"
	"go.uber.org/zap"

	"github.com/iotexproject/iotex-core/db"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

var (
	cat = []byte{1, 2, 3, 4, 5, 6, 7, 8}
)

func TestKVStoreForTrie_ErrNotExist(t *testing.T) {
	store, err := NewKVStoreForTrie("test", NewMemKVStore())
	require.NoError(t, err)
	require.NoError(t, store.Put([]byte("key"), []byte("value1")))
	require.NoError(t, store.Flush())
	// Once delete, even before flush, it should still return ErrNotExist
	require.NoError(t, store.Delete([]byte("key")))
	_, err = store.Get([]byte("key"))
	require.Equal(t, ErrNotExist, errors.Cause(err))
	// A non-existing key should return ErrNotExist too
	_, err = store.Get([]byte("key1"))
	require.Equal(t, ErrNotExist, errors.Cause(err))
}
func TestSameKey2(t *testing.T) {
	require := require.New(t)
	testTrieFile, err := ioutil.TempFile(os.TempDir(), "trie")
	require.NoError(err)

	// first trie
	cfg := config.Default.DB
	cfg.DbPath = testTrieFile.Name()

	trieDB := db.NewBoltDB(cfg)
	dbForTrie, err := db.NewKVStoreForTrie(evm.ContractKVNameSpace, trieDB, db.CachedBatchOption(db.NewCachedBatch()))
	require.NoError(err)
	log.L().Info("NewKVStoreForTrie:", zap.Error(err))
	addrHash := []byte("xx")
	options := []trie.Option{
		trie.KVStoreOption(dbForTrie),
		trie.KeyLengthOption(len(hash.Hash256{})),
		trie.HashFuncOption(func(data []byte) []byte {
			return trie.DefaultHashFunc(append(addrHash[:], data...))
		}),
	}

	options = append(options, trie.RootHashOption([]byte("")))

	tr, err := trie.NewTrie(options...)
	require.NoError(err)

	require.NoError(tr.Start(context.Background()))
	require.Nil(err)
	require.Nil(tr.Start(context.Background()))
	require.Nil(tr.Upsert(cat, []byte("xxxxx")))
	v, err := tr.Get(cat)
	require.Nil(err)
	require.Equal([]byte("xxxxx"), v)

	//save root hash
	root := make([]byte, 32)
	copy(root, tr.RootHash())

	require.Nil(tr.Upsert(cat, []byte("yyyyy")))
	v, err = tr.Get(cat)
	require.Nil(err)
	require.Equal([]byte("yyyyy"), v)

	require.NotEqual(root, tr.RootHash())
	fmt.Println("root:", hex.EncodeToString(root))
	fmt.Println("tx:", hex.EncodeToString(tr.RootHash()))
	tr.SetRootHash(root)
	v, err = tr.Get(cat)
	require.Nil(err)
	require.Equal([]byte("xxxxx"), v)
}
