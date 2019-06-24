// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
)

// accountNonceCmd represents the account nonce command
var accountNonceCmd = &cobra.Command{
	Use:   "nonce (ALIAS|ADDRESS)",
	Short: "Get nonce of an account",
Args:  cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := nonce(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// nonce gets nonce and pending nonce of an IoTeX blockchain address
func nonce(args []string) (string, error) {
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
	return fmt.Sprintf("%s:\nNonce: %d, Pending Nonce: %d",
		address, accountMeta.Nonce, accountMeta.PendingNonce), nil
}
