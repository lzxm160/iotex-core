// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"math/big"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/util"
)

// accountBalanceCmd represents the account balance command
var accountBalanceCmd = &cobra.Command{
	Use:   "balance (ALIAS|ADDRESS)",
	Short: "Get balance of an account",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := balance(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// balance gets balance of an IoTeX blockchain address
func balance(args []string) (string, error) {
	var (
		address string
		err     error
	)
	if len(args) == 1 {
		address = args[0]
	} else {
		address, err = config.GetContext()
		if err != nil {
			return "", err
		}
	}
	address, err = alias.Address(address)
	if err != nil {
		return "", err
	}
	accountMeta, err := GetAccountMeta(address)
	if err != nil {
		return "", err
	}
	balance, ok := big.NewInt(0).SetString(accountMeta.Balance, 10)
	if !ok {
		return "", fmt.Errorf("failed to convert balance into big int")
	}
	return fmt.Sprintf("%s: %s IOTX", address,
		util.RauToString(balance, util.IotxDecimalNum)), nil
}
