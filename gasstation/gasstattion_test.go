// Copyright (c) 2019 IoTeX
// This is an alpha (internal) release and is not suitable for production. This source code is provided 'as is' and no
// warranties are given as to title or non-infringement, merchantability or fitness for purpose and, to the extent
// permitted by law, all liability for your use of the code is disclaimed. This source code is governed by Apache
// License 2.0 that can be found in the LICENSE file.

package gasstation

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/iotexproject/iotex-core/pkg/unit"

	"github.com/stretchr/testify/require"

	"github.com/iotexproject/iotex-core/action"
	"github.com/iotexproject/iotex-core/action/protocol"
	"github.com/iotexproject/iotex-core/action/protocol/account"
	"github.com/iotexproject/iotex-core/action/protocol/execution"
	"github.com/iotexproject/iotex-core/action/protocol/rolldpos"
	"github.com/iotexproject/iotex-core/blockchain"
	"github.com/iotexproject/iotex-core/config"
	"github.com/iotexproject/iotex-core/pkg/version"
	"github.com/iotexproject/iotex-core/test/identityset"
	"github.com/iotexproject/iotex-core/testutil"
	"github.com/iotexproject/iotex-proto/golang/iotextypes"
)

func TestNewGasStation(t *testing.T) {
	require := require.New(t)
	require.NotNil(NewGasStation(nil, config.Default.API))
}
func TestSuggestGasPriceForUserAction(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(1000000)
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.Registry{}
	acc := account.NewProtocol(0)
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	blkState := blockchain.InMemStateFactoryOption()
	blkMemDao := blockchain.InMemDaoOption()
	blkRegistryOption := blockchain.RegistryOption(&registry)
	bc := blockchain.NewBlockchain(cfg, blkState, blkMemDao, blkRegistryOption)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	exec := execution.NewProtocol(bc, 0, 0)
	require.NoError(t, registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, exec)
	bc.GetFactory().AddActionHandlers(acc, exec)
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	for i := 0; i < 30; i++ {
		tsf, err := action.NewTransfer(
			uint64(i)+1,
			big.NewInt(100),
			identityset.Address(27).String(),
			[]byte{}, uint64(100000),
			big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev)),
		)
		require.NoError(t, err)

		bd := &action.EnvelopeBuilder{}
		elp1 := bd.SetAction(tsf).
			SetNonce(uint64(i) + 1).
			SetGasLimit(100000).
			SetGasPrice(big.NewInt(1).Mul(big.NewInt(int64(i)+10), big.NewInt(unit.Qev))).Build()
		selp1, err := action.Sign(elp1, identityset.PrivateKey(0))
		require.NoError(t, err)

		actionMap := make(map[string][]action.SealedEnvelope)
		actionMap[identityset.Address(0).String()] = []action.SealedEnvelope{selp1}

		blk, err := bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(t, err)
		require.Equal(t, 2, len(blk.Actions))
		require.Equal(t, 1, len(blk.Receipts))
		var gasConsumed uint64
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
		err = bc.ValidateBlock(blk)
		require.NoError(t, err)
		err = bc.CommitBlock(blk)
		require.NoError(t, err)
	}
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)

	gs := NewGasStation(bc, cfg.API)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(t, big.NewInt(1).Mul(big.NewInt(int64(31)), big.NewInt(unit.Qev)).Uint64(), gp)

	// test for payload with account balance is 0
	act := getActionWithPayloadWithoutBalance()
	require.NotNil(t, act)
	ret, err := gs.EstimateGasForAction(act)
	fmt.Println(ret, ":::::::", err)
	require.NoError(t, err)
	require.Equal(t, uint64(10000)+10*action.ExecutionDataGas, ret)
}

func TestSuggestGasPriceForSystemAction(t *testing.T) {
	ctx := context.Background()
	cfg := config.Default
	cfg.Genesis.BlockGasLimit = uint64(1000000)
	cfg.Genesis.EnableGravityChainVoting = false
	registry := protocol.Registry{}
	acc := account.NewProtocol(0)
	require.NoError(t, registry.Register(account.ProtocolID, acc))
	rp := rolldpos.NewProtocol(cfg.Genesis.NumCandidateDelegates, cfg.Genesis.NumDelegates, cfg.Genesis.NumSubEpochs)
	require.NoError(t, registry.Register(rolldpos.ProtocolID, rp))
	blkState := blockchain.InMemStateFactoryOption()
	blkMemDao := blockchain.InMemDaoOption()
	blkRegistryOption := blockchain.RegistryOption(&registry)
	bc := blockchain.NewBlockchain(cfg, blkState, blkMemDao, blkRegistryOption)
	bc.Validator().AddActionEnvelopeValidators(protocol.NewGenericValidator(bc))
	exec := execution.NewProtocol(bc, 0, 0)
	require.NoError(t, registry.Register(execution.ProtocolID, exec))
	bc.Validator().AddActionValidators(acc, exec)
	bc.GetFactory().AddActionHandlers(acc, exec)
	require.NoError(t, bc.Start(ctx))
	defer func() {
		require.NoError(t, bc.Stop(ctx))
	}()

	for i := 0; i < 30; i++ {
		actionMap := make(map[string][]action.SealedEnvelope)

		blk, err := bc.MintNewBlock(
			actionMap,
			testutil.TimestampNow(),
		)
		require.NoError(t, err)
		require.Equal(t, 1, len(blk.Actions))
		require.Equal(t, 0, len(blk.Receipts))
		var gasConsumed uint64
		for _, receipt := range blk.Receipts {
			gasConsumed += receipt.GasConsumed
		}
		require.True(t, gasConsumed <= cfg.Genesis.BlockGasLimit)
		err = bc.ValidateBlock(blk)
		require.NoError(t, err)
		err = bc.CommitBlock(blk)
		require.NoError(t, err)
	}
	height := bc.TipHeight()
	fmt.Printf("Open blockchain pass, height = %d\n", height)

	gs := NewGasStation(bc, cfg.API)
	require.NotNil(t, gs)

	gp, err := gs.SuggestGasPrice()
	fmt.Println(gp)
	require.NoError(t, err)
	// i from 10 to 29,gasprice for 20 to 39,60%*20+20=31
	require.Equal(t, gs.cfg.GasStation.DefaultGas, gp)
}

func TestEstimateGasForAction(t *testing.T) {
	require := require.New(t)
	act := getAction()
	require.NotNil(act)
	cfg := config.Default
	bc := blockchain.NewBlockchain(cfg, blockchain.InMemDaoOption(), blockchain.InMemStateFactoryOption())
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	gs := NewGasStation(bc, config.Default.API)
	require.NotNil(gs)
	ret, err := gs.EstimateGasForAction(act)
	require.NoError(err)
	// base intrinsic gas 10000
	require.Equal(uint64(10000), ret)

	// test for payload
	act = getActionWithPayload()
	require.NotNil(act)
	require.NoError(bc.Start(context.Background()))
	require.NotNil(bc)
	ret, err = gs.EstimateGasForAction(act)
	require.NoError(err)
	// base intrinsic gas 10000,plus data size*ExecutionDataGas
	require.Equal(uint64(10000)+10*action.ExecutionDataGas, ret)
}
func getAction() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
	}
	return
}
func getActionWithPayload() (act *iotextypes.Action) {
	pubKey1 := identityset.PrivateKey(28).PublicKey()
	addr2 := identityset.Address(29).String()

	act = &iotextypes.Action{
		Core: &iotextypes.ActionCore{
			Action: &iotextypes.ActionCore_Transfer{
				Transfer: &iotextypes.Transfer{Recipient: addr2, Payload: []byte("1234567890")},
			},
			Version: version.ProtocolVersion,
			Nonce:   101,
		},
		SenderPubKey: pubKey1.Bytes(),
	}
	return
}
func getActionWithPayloadWithoutBalance() (act *iotextypes.Action) {
	//pubKey1 := identityset.PrivateKey(20).PublicKey()
	pubKey1 := identityset.PrivateKey(0)
	exec, _ := action.NewExecution(
		"",
		30,
		big.NewInt(0),
		10000,
		big.NewInt(10),
		[]byte("608060405234801561001057600080fd5b50604051608080610791833981016040908152815160208301519183015160609093015160008054600160a060020a03191633179055909290600160a060020a038416151561005e57600080fd5b6000821161006b57600080fd5b8181101561007857600080fd5b60018054600160a060020a031916600160a060020a039590951694909417909355600291909155600355600455623d09006005556106d6806100bb6000396000f3006080604052600436106100fb5763ffffffff7c0100000000000000000000000000000000000000000000000000000000600035041663046f7da28114610105578063186f03541461011a5780632e1a7d4d1461014b57806345f0a44f14610163578063490ae2101461018d5780634fe47f70146101a55780635db0cb94146101bd5780635f48f393146101de57806367a52793146101f35780638456cb5914610208578063897b06371461021d5780638da5cb5b146102355780639b2cb5d81461024a578063c0abda2a1461025f578063d0e30db0146100fb578063ee7d72b414610277578063f2fde38b1461028f578063f68016b7146102b0575b6101036102c5565b005b34801561011157600080fd5b506101036103f0565b34801561012657600080fd5b5061012f610424565b60408051600160a060020a039092168252519081900360200190f35b34801561015757600080fd5b50610103600435610433565b34801561016f57600080fd5b5061017b600435610489565b60408051918252519081900360200190f35b34801561019957600080fd5b506101036004356104a8565b3480156101b157600080fd5b506101036004356104c4565b3480156101c957600080fd5b50610103600160a060020a03600435166104ef565b3480156101ea57600080fd5b5061017b61054a565b3480156101ff57600080fd5b5061017b610550565b34801561021457600080fd5b50610103610556565b34801561022957600080fd5b5061010360043561058c565b34801561024157600080fd5b5061012f6105b7565b34801561025657600080fd5b5061017b6105c6565b34801561026b57600080fd5b5061012f6004356105cc565b34801561028357600080fd5b506101036004356105f4565b34801561029b57600080fd5b50610103600160a060020a0360043516610610565b3480156102bc57600080fd5b5061017b6106a4565b60085460009060ff16156102d857600080fd5b600254600354013410156102eb57600080fd5b60025434039050600454811115151561030357600080fd5b600154600554604051600160a060020a039092169183906000818181858888f19350505050156103ed576006805460018181019092557ff652222313e28459528d920b65115c16c04f3efc82aaedc97be59f3f377c0d3f01805473ffffffffffffffffffffffffffffffffffffffff1916339081179091556007805492830181556000527fa66cc928b5edb82af9bd49922954155ab7b0942694bea4ce44661d9a8736c68890910182905560025460408051848152602081019290925280517fb54144b2711919f9fb59c30ec3b593b154784e26488806a6ddb320c41b5c1c939281900390910190a25b50565b600054600160a060020a0316331461040757600080fd5b60085460ff16151561041857600080fd5b6008805460ff19169055565b600154600160a060020a031681565b600054600160a060020a0316331461044a57600080fd5b303181111561045857600080fd5b604051339082156108fc029083906000818181858888f19350505050158015610485573d6000803e3d6000fd5b5050565b600780548290811061049757fe5b600091825260209091200154905081565b600054600160a060020a031633146104bf57600080fd5b600255565b600054600160a060020a031633146104db57600080fd5b6003548110156104ea57600080fd5b600455565b600054600160a060020a0316331461050657600080fd5b600160a060020a038116151561051b57600080fd5b6001805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0392909216919091179055565b60045481565b60025481565b600054600160a060020a0316331461056d57600080fd5b60085460ff161561057d57600080fd5b6008805460ff19166001179055565b600054600160a060020a031633146105a357600080fd5b6004548111156105b257600080fd5b600355565b600054600160a060020a031681565b60035481565b60068054829081106105da57fe5b600091825260209091200154600160a060020a0316905081565b600054600160a060020a0316331461060b57600080fd5b600555565b600054600160a060020a0316331461062757600080fd5b600160a060020a038116151561063c57600080fd5b60008054604051600160a060020a03808516939216917f8be0079c531659141344cd1fd0a4f28419497f9722a3daafe3b4186f6b6457e091a36000805473ffffffffffffffffffffffffffffffffffffffff1916600160a060020a0392909216919091179055565b600554815600a165627a7a72305820d8278a1efc7155cd5660666a40251a6011e1387cb8adbb4bfdcc17082890c1c00029000000000000000000000000cecc938840c5ae89373a681a5f2e0f244152e91b000000000000000000000000000000000000000000000000000000000000271000000000000000000000000000000000000000000000000000000000000186a000000000000000000000000000000000000000000000000000000000000f4240"),
	)
	builder := &action.EnvelopeBuilder{}
	elp := builder.SetAction(exec).
		SetNonce(exec.Nonce()).
		SetGasLimit(exec.GasLimit()).
		SetGasPrice(exec.GasPrice()).
		Build()
	selp, _ := action.Sign(elp, pubKey1)

	act = &iotextypes.Action{
		Core:         selp.Proto().Core,
		SenderPubKey: pubKey1.PublicKey().Bytes(),
	}
	return
}
