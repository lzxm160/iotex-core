// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/hex"
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
	getHashCmdUses = map[config.Language]string{
		config.English: "gethash (CONTRACT_ADDRESS|ALIAS) hash",
		config.Chinese: "gethash (合约地址|别名) hash",
	}
	getHashCmdShorts = map[config.Language]string{
		config.English: "gethash get DID doc's hash on IoTeX blockchain",
		config.Chinese: "gethash 在IoTeX链上获取相应DID的doc hash",
	}
)

// didGetHashCmd represents the contract invoke getHash command
var didGetHashCmd = &cobra.Command{
	Use:   config.TranslateInLang(getHashCmdUses, config.UILanguage),
	Short: config.TranslateInLang(getHashCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		result, err := getHash(args)
		if err != nil {
			return output.PrintError(err)
		}
		output.PrintResult(result)
	},
}

func init() {
	action.RegisterWriteCommand(didGetHashCmd)
}

func getHash(args []string) (ret string, err error) {
	contract, err := util.Address(args[0])
	if err != nil {
		err = output.NewError(output.AddressError, "failed to get contract address", err)
		return
	}

	bytecode, err := encodeGetHash(args[1])
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

func encodeGetHash(didHash string) (ret []byte, err error) {
	hashSlice, err := hex.DecodeString(didHash)
	if err != nil {
		return
	}
	var hashArray [32]byte
	copy(hashArray[:], hashSlice)
	abi, err := abi.JSON(strings.NewReader(AddressBasedDIDManagerABI))
	if err != nil {
		return
	}
	_, exist := abi.Methods[getHashName]
	if !exist {
		return nil, errors.New("method is not found")
	}
	return abi.Pack(getHashName, hashArray)
}

//
//func getPrivate() (crypto.PrivateKey, error) {
//	addr, err := action.Signer()
//	if err != nil {
//		return nil, output.NewError(output.InputError, "failed to get signer addr", err)
//	}
//	fmt.Printf("Enter password #%s:\n", addr)
//	password, err := util.ReadSecretFromStdin()
//	if err != nil {
//		return nil, output.NewError(output.InputError, "failed to get password", err)
//	}
//	return account.LocalAccountToPrivateKey(addr, password)
//}
