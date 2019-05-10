// Copyright (c) 2018 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package protocol

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-address/address"
	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/test/mock/mock_chainmanager"
	"github.com/iotexproject/iotex-core/test/testaddress"
)

func TestActionProto(t *testing.T) {
	require := require.New(t)
	caller, err := address.FromString("io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6")
	require.NoError(err)
	ctx := ValidateActionsCtx{1, "io1emxf8zzqckhgjde6dqd97ts0y3q496gm3fdrl6", caller}
	c := WithValidateActionsCtx(context.Background(), ctx)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()
	mcm := mock_chainmanager.NewMockChainManager(ctrl)
	valid := NewGenericValidator(mcm, 100000)
	data, err := hex.DecodeString("")
	require.NoError(err)
	v, err := action.NewExecution("", 0, big.NewInt(10), uint64(10), big.NewInt(10), data)
	require.NoError(err)
	fmt.Println(v)

	bd := &action.EnvelopeBuilder{}
	elp := bd.SetGasPrice(big.NewInt(10)).
		SetGasLimit(uint64(100000)).
		SetAction(v).Build()

	selp, err := action.Sign(elp, testaddress.Keyinfo["alfa"].PriKey)
	require.NoError(err)

	nselp := action.SealedEnvelope{}
	require.NoError(nselp.LoadProto(selp.Proto()))

	require.NoError(valid.Validate(c, nselp))
}
