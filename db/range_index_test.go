// Copyright (c) 2019 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package db

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/testutil"
)

func TestRangeIndex(t *testing.T) {
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
	index, err := kv.CreateRangeIndexNX(testNS)
	require.NoError(err)

	err = index.Insert(7, []byte("7"))
	require.NoError(err)
	// Case I: key before 7
	for i := uint64(1); i < 6; i++ {
		index, err = kv.CreateRangeIndexNX(testNS)
		require.NoError(err)
		_, err = index.Get(i)
		require.Error(err)
	}
	// Case II: key is 7 and greater than 7
	for i := uint64(7); i < 10; i++ {
		index, err = kv.CreateRangeIndexNX(testNS)
		require.NoError(err)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal([]byte("7"), v)
	}
	// Case III: duplicate key
	index, err = kv.CreateRangeIndexNX(testNS)
	require.NoError(err)
	err = index.Insert(7, []byte("7777"))
	require.NoError(err)
	for i := uint64(7); i < 10; i++ {
		index, err = kv.CreateRangeIndexNX(testNS)
		v, err := index.Get(i)
		require.NoError(err)
		require.Equal([]byte("7777"), v)
	}
	// Case IV: delete key less than 7
	for i := uint64(1); i < 7; i++ {
		index, err = kv.CreateRangeIndexNX(testNS)
		err = index.Delete(i)
		require.NoError(err)
	}
	index, err = kv.CreateRangeIndexNX(testNS)
	require.NoError(err)
	v, err := index.Get(7)
	require.NoError(err)
	require.Equal([]byte("7777"), v)

	//v, err = index.Get(8)
	//fmt.Println(string(v), ":", err)
	//
	//index.Close()
	//
	//index2, err := kv.CreateRangeIndexNX(testNS)
	//require.NoError(err)
	//
	//err = index2.Insert(20, []byte("20"))
	//fmt.Println(err)
	//for i := 7; i < 22; i++ {
	//	v, err := index2.Get(uint64(i))
	//	fmt.Println(string(v), ":", err)
	//}
	//for i, e := range rangeTests {
	//	if i == 0 {
	//		continue
	//	}
	//	require.NoError(index.Insert(e.k, e.v))
	//	// test 5 random keys between the new and previous insertion
	//	gap := e.k - rangeTests[i-1].k
	//	for j := 0; j < 5; j++ {
	//		k := rangeTests[i-1].k + uint64(rand.Intn(int(gap)))
	//		v, err := index.Get(k)
	//		require.NoError(err)
	//		require.Equal(rangeTests[i-1].v, v)
	//		fmt.Println(k, ":", string(v))
	//	}
	//	v, err := index.Get(e.k - 1)
	//	require.NoError(err)
	//	require.Equal(rangeTests[i-1].v, v)
	//	v, err = index.Get(e.k)
	//	require.NoError(err)
	//	require.Equal(e.v, v)
	//	fmt.Println(e.k, ":", string(v))
	//	// test 5 random keys beyond new insertion
	//	for j := 0; j < 5; j++ {
	//		k := e.k + uint64(rand.Int())
	//		v, err := index.Get(k)
	//		require.NoError(err)
	//		require.Equal(e.v, v)
	//		fmt.Println(k, ":", string(v))
	//	}
	//}
	//
	//for j := uint64(0); j <= 100; j++ {
	//	if j > 30 && j < 90 {
	//		continue
	//	}
	//	v, _ := index.Get(j)
	//	fmt.Println(j, ":", string(v))
	//}
	//fmt.Println("---------------------------")
	//err = index.Delete(6)
	//fmt.Println(err)
	//for j := uint64(0); j <= 100; j++ {
	//	if j > 30 && j < 90 {
	//		continue
	//	}
	//	v, err := index.Get(j)
	//	fmt.Println(j, ":", string(v), ":", err)
	//}
	//fmt.Println("---------------------------")
	//err = index.Delete(50)
	//fmt.Println(err)
	//for j := uint64(0); j <= 100; j++ {
	//	if j > 30 && j < 90 {
	//		continue
	//	}
	//	v, err := index.Get(j)
	//	fmt.Println(j, ":", string(v), ":", err)
	//}
}
