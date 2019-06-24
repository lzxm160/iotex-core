// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package alias

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/cli/ioctl/cmd/config"
	"github.com/iotexproject/iotex-core/cli/ioctl/validator"
)

// aliasSetCmd represents the alias set command
var aliasSetCmd = &cobra.Command{
	Use:   "set ALIAS ADDRESS",
	Short: "Set alias for address",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := set(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// set sets alias
func set(args []string) (string, error) {
	var (
		alias   string
		address string
		err     error
	)
	if len(args) == 2 {
		alias = args[0]
		address = args[1]
	} else {
		address = args[0]
		alias, err = config.GetContext()
		if err != nil {
			return "", err
		}
	}
	if err := validator.ValidateAlias(alias); err != nil {
		return "", err
	}
	if err := validator.ValidateAddress(address); err != nil {
		return "", err
	}
	addr := args[1]
	aliases := GetAliasMap()
	if aliases[addr] != "" {
		delete(config.ReadConfig.Aliases, aliases[addr])
	}
	config.ReadConfig.Aliases[alias] = addr
	out, err := yaml.Marshal(&config.ReadConfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(config.DefaultConfigFile, out, 0600); err != nil {
		return "", fmt.Errorf("failed to write to config file %s", config.DefaultConfigFile)
	}
	return "set", nil
}
