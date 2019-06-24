// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/alias"
	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountExportCmd represents the account export command
var accountExportCmd = &cobra.Command{
	Use:   "export (ALIAS|ADDRESS)",
	Short: "Export IoTeX private key from wallet",
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := accountExport(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func accountExport(args []string) (string, error) {
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
	addr, err := alias.Address(address)
	if err != nil {
		return "", err
	}
	fmt.Printf("Enter password #%s:\n", address)
	bytePassword, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	prvKey, err := KsAccountToPrivateKey(addr, string(bytePassword))
	if err != nil {
		return "", err
	}
	defer prvKey.Zero()
	return prvKey.HexString(), nil
}
