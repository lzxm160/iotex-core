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

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/action"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

const (
	registerDIDName           = "registerDID"
	AddressBasedDIDManagerABI = `[{"inputs": [{"internalType": "bytes","name": "_prefix","type": "bytes"},{"internalType": "address","name": "_dbAddr","type": "address"}],"payable": false,"stateMutability": "nonpayable","type": "constructor"},{"constant": false,"inputs": [],"name": "deregisterDID","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},{"constant": true,"inputs": [{"internalType": "bytes","name": "did","type": "bytes"}],"name": "getHash","outputs": [{"internalType": "bytes32","name": "","type": "bytes32"}],"payable": false,"stateMutability": "view","type": "function"},   {"constant": true,"inputs": [{"internalType": "bytes","name": "did","type": "bytes"}],"name": "getURI","outputs": [{"internalType": "bytes","name": "","type": "bytes"}],"payable": false,"stateMutability": "view","type": "function"},{"constant": false,"inputs": [{"internalType": "bytes32","name": "h","type": "bytes32"},{"internalType": "bytes","name": "uri","type": "bytes"}],"name": "registerDID","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"},{"constant": false,"inputs": [{"internalType": "bytes32","name": "h","type": "bytes32"},{"internalType": "bytes","name": "uri","type": "bytes"}],"name": "updateDID","outputs": [],"payable": false,"stateMutability": "nonpayable","type": "function"}]`
)

// Multi-language support
var (
	registerCmdUses = map[config.Language]string{
		config.English: "register (CONTRACT_ADDRESS|ALIAS) hash uri",
		config.Chinese: "register (合约地址|别名) hash uri",
	}
	registerCmdShorts = map[config.Language]string{
		config.English: "register DID on IoTeX blockchain",
		config.Chinese: "register 在IoTeX链上注册DID",
	}
)

// didRegisterCmd represents the contract invoke register command
var didRegisterCmd = &cobra.Command{
	Use:   config.TranslateInLang(registerCmdUses, config.UILanguage),
	Short: config.TranslateInLang(registerCmdShorts, config.UILanguage),
	Args:  cobra.RangeArgs(3, 4),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := registerDID(args)
		return output.PrintError(err)
	},
}

func init() {
	action.RegisterWriteCommand(didRegisterCmd)
}

func registerDID(args []string) error {
	contract, err := util.Address(args[0])
	if err != nil {
		return output.NewError(output.AddressError, "failed to get contract address", err)
	}

	bytecode, err := encode(args[1], args[2])
	if err != nil {
		return output.NewError(output.ConvertError, "invalid bytecode", err)
	}

	return action.Execute(contract, big.NewInt(0), bytecode)
}

func encode(didHash, uri string) (ret []byte, err error) {
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
	_, exist := abi.Methods[registerDIDName]
	if !exist {
		return nil, errors.New("method is not found")
	}
	return abi.Pack(registerDIDName, hashArray, []byte(uri))
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
