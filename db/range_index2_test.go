// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestRangeIndex2(t *testing.T) {
	require := require.New(t)

	//	{0, []byte{0}},
	//	{1, []byte("1")},
	//	{7, []byte("7")},
	//	{29, []byte("29")},
	//	{100, []byte("100")},

	path := "test-indexer"
	testFile, _ := ioutil.TempFile(os.TempDir(), path)
	testPath := testFile.Name()
	cfg := config.Default.DB
	cfg.DbPath = testPath
	testutil.CleanupPath(t, testPath)
	defer testutil.CleanupPath(t, testPath)

	kv := NewBoltDB(cfg)
	require.NotNil(kv)

	require.NoError(kv.Start(context.Background()))
	defer func() {
		require.NoError(kv.Stop(context.Background()))
	}()

	testNS := []byte("test")
	index, err := kv.CreateRangeIndexNX(testNS, []byte{})
	require.NoError(err)

	err = index.Insert(7, []byte("7"))
	require.NoError(err)
	// Case I: key before 7
	for i := uint64(1); i < 6; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal(v, NotExist)
	}
	// Case II: key is 7 and greater than 7
	for i := uint64(7); i < 10; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal([]byte("7"), v)
	}
	// Case III: duplicate key
	index, err = kv.CreateRangeIndexNX(testNS, []byte{})
	require.NoError(err)
	err = index.Insert(7, []byte("7777"))
	require.NoError(err)
	for i := uint64(7); i < 10; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal([]byte("7777"), v)
	}
	// Case IV: delete key less than 7
	index, err = kv.CreateRangeIndexNX(testNS, []byte{})
	require.NoError(err)
	err = index.Insert(66, []byte("66"))
	require.NoError(err)
	for i := uint64(1); i < 7; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		err = index.Delete(i)
		require.NoError(err)
	}
	index, err = kv.CreateRangeIndexNX(testNS, []byte{})
	require.NoError(err)
	v, err := index.Get(7)
	require.NoError(err)
	require.Equal([]byte("7777"), v)
	// Case V: delete key 7
	index, err = kv.CreateRangeIndexNX(testNS, []byte{})
	err = index.Purge(10)
	require.NoError(err)
	for i := uint64(1); i < 66; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		require.NoError(err)
		v, err := index.Get(i)
		fmt.Println(i, ":", string(v), ":", err)
		//require.Error(err)
		//require.NoError(err)
		//require.Equal(v, NotExist)
	}
	for i := uint64(66); i < 70; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		require.NoError(err)
		v, err = index.Get(i)
		require.Equal([]byte("66"), v)
	}
	// Case VI: delete key before 80,all keys deleted
	index, err = kv.CreateRangeIndexNX(testNS, []byte{})
	err = index.Insert(70, []byte("70"))
	require.NoError(err)
	index, err = kv.CreateRangeIndexNX(testNS, []byte{})
	err = index.Insert(80, []byte("80"))
	require.NoError(err)
	index, err = kv.CreateRangeIndexNX(testNS, []byte{})
	err = index.Delete(79)
	require.NoError(err)
	for i := uint64(1); i < 80; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal(v, NotExist)
	}
	for i := uint64(80); i < 90; i++ {
		index, err = kv.CreateRangeIndexNX(testNS, []byte{})
		require.NoError(err)
		v, err = index.Get(i)
		require.NoError(err)
		require.Equal([]byte("80"), v)
	}
}
