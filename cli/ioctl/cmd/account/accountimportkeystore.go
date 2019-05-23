// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package account

import (
	"fmt"
	"io/ioutil"
	"strings"
	"syscall"

	"go.uber.org/zap"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
	"github.com/iotexproject/iotex-core/pkg/log"
)

// accountImportKeyStoreCmd represents the account import keystore command
var accountImportKeyStoreCmd = &cobra.Command{
	Use:   "importkeystore ALIAS PATH",
	Short: "Import IoTeX keystore into wallet",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := accountImportKeyStore(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func accountImportKeyStore(args []string) (string, error) {
	// Validate inputs
	if err := validator.ValidateAlias(args[0]); err != nil {
		return "", err
	}
	alias := args[0]
	if addr, ok := config.ReadConfig.Aliases[alias]; ok {
		return "", fmt.Errorf("alias \"%s\" has already used for %s", alias, addr)
	}
	fmt.Printf("#%s: Enter your password of keystore, which will not be exposed on the screen.\n", alias)
	passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.L().Error("failed to get password", zap.Error(err))
		return "", err
	}
	password := strings.TrimSpace(string(passwordBytes))
	for i := 0; i < len(passwordBytes); i++ {
		passwordBytes[i] = 0
	}
	addr, err := newAccountByKeyStore(alias, password, args[1], config.ReadConfig.Wallet)
	if err != nil {
		return "", err
	}
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return "", fmt.Errorf("failed to write to config file %s", config.DefaultConfigFile)
	}
	return fmt.Sprintf(
		"New account #%s is created. Keep your password, or your will lose your private key.",
		alias), nil
}
