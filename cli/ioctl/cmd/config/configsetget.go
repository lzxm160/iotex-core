// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package config

import (
	"fmt"
	"io/ioutil"
	"regexp"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	validArgs = []string{"endpoint", "wallet"}
)

// configGetCmd represents the config get command
var configGetCmd = &cobra.Command{
	Use:       "get VARIABLE",
	Short:     "Get config from ioctl",
	ValidArgs: validArgs,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return fmt.Errorf("accepts 1 arg(s), received %d,"+
				" valid arg(s): %s", len(args), validArgs)
		}
		return cobra.OnlyValidArgs(cmd, args)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := Get(args[0])
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

// configSetCmd represents the config set command
var configSetCmd = &cobra.Command{
	Use:       "set VARIABLE VALUE",
	Short:     "Set config for ioctl",
	ValidArgs: []string{"endpoint", "wallet"},
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 2 {
			return fmt.Errorf("accepts 2 arg(s), received %d,"+
				" valid arg(s): %s", len(args), validArgs)
		}
		return cobra.OnlyValidArgs(cmd, args[:1])
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		output, err := set(args)
		if err == nil {
			fmt.Println(output)
		}
		return err
	},
}

func init() {
	configSetCmd.Flags().BoolVar(&Insecure, "insecure", false,
		"set insecure connection as default")
}

// Get gets config variable
func Get(arg string) (string, error) {
	switch arg {
	default:
		return "", ErrConfigNotMatch
	case "endpoint":
		if ReadConfig.Endpoint == "" {
			return "", ErrEmptyEndpoint
		}
		return fmt.Sprint(ReadConfig.Endpoint, "    secure connect(TLS):",
			ReadConfig.SecureConnect), nil
	case "wallet":
		return ReadConfig.Wallet, nil
	}
}
func isMatch(endpoint string)bool{
	fmt.Println(endpoint)
	const(
		ip4Pattern = `((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)`
		ip6Pattern = `(([0-9A-Fa-f]{1,4}:){7}([0-9A-Fa-f]{1,4}|:))|` +
			`(([0-9A-Fa-f]{1,4}:){6}(:[0-9A-Fa-f]{1,4}|((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|` +
			`(([0-9A-Fa-f]{1,4}:){5}(((:[0-9A-Fa-f]{1,4}){1,2})|:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3})|:))|` +
			`(([0-9A-Fa-f]{1,4}:){4}(((:[0-9A-Fa-f]{1,4}){1,3})|((:[0-9A-Fa-f]{1,4})?:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|` +
			`(([0-9A-Fa-f]{1,4}:){3}(((:[0-9A-Fa-f]{1,4}){1,4})|((:[0-9A-Fa-f]{1,4}){0,2}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|` +
			`(([0-9A-Fa-f]{1,4}:){2}(((:[0-9A-Fa-f]{1,4}){1,5})|((:[0-9A-Fa-f]{1,4}){0,3}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|` +
			`(([0-9A-Fa-f]{1,4}:){1}(((:[0-9A-Fa-f]{1,4}){1,6})|((:[0-9A-Fa-f]{1,4}){0,4}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))|` +
			`(:(((:[0-9A-Fa-f]{1,4}){1,7})|((:[0-9A-Fa-f]{1,4}){0,5}:((25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)(\.(25[0-5]|2[0-4]\d|1\d\d|[1-9]?\d)){3}))|:))`
		ipPattern = "(" + ip4Pattern + ")|(" + ip6Pattern + ")"
		domainPattern = `[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}(\.[a-zA-Z0-9][a-zA-Z0-9_-]{0,62})*(\.[a-zA-Z][a-zA-Z0-9]{0,10}){1}`
		endpointPattern ="(" + ipPattern + "|(" + domainPattern + "))" +`(:\d{1,5})?`
	)
	endpointCompile,err:= regexp.Compile(endpointPattern)
	if err!=nil{
		return false
	}
	return endpointCompile.MatchString(endpoint)
}
// set sets config variable
func set(args []string) (string, error) {
	switch args[0] {
	default:
		return "", ErrConfigNotMatch
	case "endpoint":
		if !isMatch(args[1]){
			return "",fmt.Errorf("endpoint %s match error", args[1])
		}
		ReadConfig.Endpoint = args[1]
		ReadConfig.SecureConnect = !Insecure
	case "wallet":
		ReadConfig.Wallet = args[1]
	}
	out, err := yaml.Marshal(&ReadConfig)
	if err != nil {
		return "", err
	}
	if err := ioutil.WriteFile(DefaultConfigFile, out, 0600); err != nil {
		return "", fmt.Errorf("failed to write to config file %s", DefaultConfigFile)
	}
	return args[0] + " is set to " + args[1], nil
}
