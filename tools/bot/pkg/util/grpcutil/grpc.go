package grpcutil

import (
	"context"
	"errors"

	"github.com/iotexproject/go-pkgs/crypto"
	"github.com/iotexproject/go-pkgs/hash"
	"github.com/iotexproject/iotex-antenna-go/v2/account"
	"github.com/iotexproject/iotex-antenna-go/v2/iotex"
	"github.com/iotexproject/iotex-proto/golang/iotexapi"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
	"google.golang.org/grpc"

	"github.com/iotexproject/iotex-core/action"
)

// ConnectToEndpoint connect to endpoint
func ConnectToEndpoint(url string) (*grpc.ClientConn, error) {
	//endpoint := url
	//if endpoint == "" {
	//	return nil, errors.New(`endpoint is empty`)
	//}
	//return grpc.Dial(endpoint, grpc.WithInsecure())
	return iotex.NewDefaultGRPCConn(url)
}

// GetReceiptByActionHash get receipt by action hash
func GetReceiptByActionHash(url string, hs string) error {
	conn, err := ConnectToEndpoint(url)
	if err != nil {
		return err
	}
	defer conn.Close()
	c := iotexapi.NewAPIServiceClient(conn)
	if c == nil {
		return errors.New("NewAPIServiceClient error")
	}
	cli := iotex.NewReadOnlyClient(c)

	hash, err := hash.HexStringToHash256(hs)
	if err != nil {
		return err
	}
	caller := cli.GetReceipt(hash)
	response, err := caller.Call(context.Background())

	if response.ReceiptInfo.Receipt.Status != action.SuccessReceiptStatus {
		return errors.New("action fail:" + hs)
	}
	return nil
}
func GetAuthedClient(url string, pri crypto.PrivateKey) (cli iotex.AuthedClient, err error) {
	conn, err := ConnectToEndpoint(url)
	if err != nil {
		return
	}
	defer conn.Close()
	acc, err := account.PrivateKeyToAccount(pri)
	if err != nil {
		return
	}
	cli = iotex.NewAuthedClient(iotexapi.NewAPIServiceClient(conn), acc)
	return
}

// SendAction send action to endpoint
func SendAction(url string, action *iotextypes.Action) error {
	conn, err := ConnectToEndpoint(url)
	if err != nil {
		return err
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	req := &iotexapi.SendActionRequest{Action: action}
	if _, err = cli.SendAction(context.Background(), req); err != nil {
		return err
	}
	return nil
}

// GetNonce get nonce of address
func GetNonce(url string, address string) (nonce uint64, err error) {
	conn, err := ConnectToEndpoint(url)
	if err != nil {
		return
	}
	defer conn.Close()
	cli := iotexapi.NewAPIServiceClient(conn)
	request := iotexapi.GetAccountRequest{Address: address}
	response, err := cli.GetAccount(context.Background(), &request)
	if err != nil {
		return
	}
	nonce = response.AccountMeta.PendingNonce
	return
}
