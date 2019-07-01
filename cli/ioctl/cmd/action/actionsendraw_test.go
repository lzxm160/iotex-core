// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package action

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"

	"github.com/iotexproject/iotex-core/action"

	"github.com/stretchr/testify/require"
)

func TestSendRaw(t *testing.T) {
	require := require.New(t)
	nonce := uint64(0)
	amount := big.NewInt(1000000)
	receipt := "io1eyn9tc6t782zx4zgy3hgt32hpz6t8v7pgf524z"
	gaslimit := uint64(10000)
	gasprice := big.NewInt(100000)
	tx, err := action.NewTransfer(nonce, amount,
		receipt, nil, gaslimit, gasprice)
	require.NoError(err)
	elp := (&action.EnvelopeBuilder{}).
		SetNonce(nonce).
		SetGasPrice(gasprice).
		SetGasLimit(gaslimit).
		SetAction(tx).Build()
	ks := keystore.NewKeyStore(config.ReadConfig.Wallet,
		keystore.StandardScryptN, keystore.StandardScryptP)
	require.NotNil(ks)

	// create an account
	passwd := "3dj,<>@@SF{}rj0ZF#"
	acc, err := ks.NewAccount(passwd)
	require.NoError(err)
	pri, err := crypto.KeystoreToPrivateKey(acc, passwd)
	require.NoError(err)
	sealed, err := action.Sign(elp, pri)
	act := sealed.Proto()
	act.Signature[64] = act.Signature[64] + 27
	require.Error(sendRaw(act)) //connect error
}
