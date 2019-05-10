// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"testing"
	"time"

	"github.com/iotexproject/go-pkgs/hash"

	"github.com/iotexproject/iotex-address/address"

	"github.com/stretchr/testify/require"
)

func TestWithRunActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := RunActionsCtx{1, time.Now(), 1, addr, addr, hash.ZeroHash256, 0, nil, 0, 0, nil}
	require.NotNil(WithRunActionsCtx(context.Background(), actionCtx))
}
func TestGetRunActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := RunActionsCtx{1111, time.Now(), 1, addr, addr, hash.ZeroHash256, 0, nil, 0, 0, nil}
	ctx := WithRunActionsCtx(context.Background(), actionCtx)
	require.NotNil(ctx)
	ret, ok := GetRunActionsCtx(ctx)
	require.True(ok)
	require.Equal(1111, ret.BlockHeight)
}
func TestMustGetRunActionsCtx(t *testing.T) {
	require := require.New(t)
	addr, err := address.FromString("io1mflp9m6hcgm2qcghchsdqj3z3eccrnekx9p0ms")
	require.NoError(err)
	actionCtx := RunActionsCtx{1111, time.Now(), 1, addr, addr, hash.ZeroHash256, 0, nil, 0, 0, nil}
	ctx := WithRunActionsCtx(context.Background(), actionCtx)
	require.NotNil(ctx)
	// Case I: Normal
	ret := MustGetRunActionsCtx(ctx)
	require.Equal(1111, ret.BlockHeight)
	// Case II: Panic
	require.Panics(func() { MustGetRunActionsCtx(context.Background()) }, "Miss run actions context")
}
