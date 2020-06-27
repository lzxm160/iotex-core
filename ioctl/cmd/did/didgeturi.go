// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"errors"
	"math/big"
	"strings"

	"github.com/iotexproject/iotex-address/address"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

// Multi-language support
var (
	getURICmdUses = map[config.Language]string{
		config.English: "geturi (CONTRACT_ADDRESS|ALIAS) uri",
		config.Chinese: "geturi (合约地址|别名) uri",
	}
	getURICmdShorts = map[config.Language]string{
		config.English: "geturi get DID uri on IoTeX blockchain",
		config.Chinese: "geturi 在IoTeX链上获取相应DID的uri",
	}
)

// didGetURICmd represents the contract invoke getURI command
var didGetURICmd = &cobra.Command{
	Use:   config.TranslateInLang(getURICmdUses, config.UILanguage),
	Short: config.TranslateInLang(getURICmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		result, err := getURI(args)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(result)
	},
}

func init() {
	action.RegisterWriteCommand(didGetURICmd)
}

func getURI(args []string) (ret string, err error) {
	contract, err := util.Address(args[0])
	if err != nil {
		err = output.NewError(output.AddressError, "failed to get contract address", err)
		return
	}

	bytecode, err := encodeGetURI(args[1])
	if err != nil {
		err = output.NewError(output.ConvertError, "invalid bytecode", err)
		return
	}
	addr, err := address.FromString(contract)
	if err != nil {
		err = output.NewError(output.ConvertError, "invalid contract address", err)
		return
	}
	return action.Read(addr, big.NewInt(0), bytecode)
}

func encodeGetURI(uri string) (ret []byte, err error) {
	abi, err := abi.JSON(strings.NewReader(AddressBasedDIDManagerABI))
	if err != nil {
		return
	}
	_, exist := abi.Methods[getURIName]
	if !exist {
		return nil, errors.New("method is not found")
	}
	return abi.Pack(getURIName, []byte(uri))
}
