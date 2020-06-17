// Copyright (c) 2020 IoTeX Foundation
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package did

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iotexproject/iotex-core/ioctl/cmd/account"
	"github.com/iotexproject/iotex-core/ioctl/config"
	"github.com/iotexproject/iotex-core/ioctl/output"
	"github.com/iotexproject/iotex-core/ioctl/util"
)

var signer string

// Multi-language support
var (
	generateCmdShorts = map[config.Language]string{
		config.English: "Generate DID document using private key from wallet",
		config.Chinese: "用钱包中的私钥产生DID document",
	}
	generateCmdUses = map[config.Language]string{
		config.English: "sign MESSAGE [-s SIGNER]",
		config.Chinese: "sign 信息 [-s 签署人]",
	}
	flagSignerUsages = map[config.Language]string{
		config.English: "choose a signing account",
		config.Chinese: "选择一个签名账户",
	}
)

// generateCmd represents the account sign command
var generateCmd = &cobra.Command{
	Use:   config.TranslateInLang(generateCmdUses, config.UILanguage),
	Short: config.TranslateInLang(generateCmdShorts, config.UILanguage),
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		err := generate()
		return output.PrintError(err)
	},
}

func init() {
	generateCmd.Flags().StringVarP(&signer, "signer", "s", "", config.TranslateInLang(flagSignerUsages, config.UILanguage))
}

func generate() error {
	addr, err := util.GetAddress(signer)
	if err != nil {
		return output.NewError(output.InputError, "failed to get signer addr", err)
	}
	fmt.Printf("Enter password #%s:\n", addr)
	password, err := util.ReadSecretFromStdin()
	if err != nil {
		return output.NewError(output.InputError, "failed to get password", err)
	}
	generatedMessage, err := generateFromSigner(addr, password)
	if err != nil {
		return output.NewError(output.KeystoreError, "failed to sign message", err)
	}
	output.PrintResult(generatedMessage)
	return nil
}

func generateFromSigner(signer, password string) (generatedMessage string, err error) {
	pri, err := account.LocalAccountToPrivateKey(signer, password)
	if err != nil {
		return
	}
	doc := newDIDDoc()
	ethAddress, err := util.IoAddrToEvmAddr(signer)
	if err != nil {
		return "", output.NewError(output.AddressError, "", err)
	}
	doc.Id = DIDPrefix + ethAddress.String()

	authentication := authenticationStruct{
		Id:           doc.Id,
		Type:         "Secp256k1VerificationKey2018",
		Controller:   doc.Id,
		PublicKeyHex: pri.PublicKey().HexString(),
	}
	doc.Authentication = append(doc.Authentication, authentication)
	msg, err := json.Marshal(doc)
	if err != nil {
		return "", output.NewError(output.ConvertError, "", err)
	}
	generatedMessage = string(msg)
	return
}
