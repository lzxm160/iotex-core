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

func TestRangeIndex(t *testing.T) {
	require := require.New(t)

	rangeTests := []struct {
		k uint64
		v []byte
	}{
		{0, []byte{0}},
		{1, []byte("1")},
		{7, []byte("7")},
		{29, []byte("29")},
		{100, []byte("100")},
	}

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

	index, err := kv.CreateRangeIndexNX([]byte("test"))
	require.NoError(err)
	one, err := index.Get(1)
	require.NoError(err)
	require.Equal(rangeTests[0].v, one)

	err = index.Insert(7, []byte("7"))
	fmt.Println(err)
	v, err := index.Get(7)
	fmt.Println(string(v), ":", err)
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
