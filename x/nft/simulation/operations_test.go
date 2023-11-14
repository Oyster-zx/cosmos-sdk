package simulation_test

import (
	"math/rand"
	"testing"
	"time"

	"github.com/cosmos/gogoproto/proto"
	"github.com/stretchr/testify/suite"

	"cosmossdk.io/core/header"
	"cosmossdk.io/depinject"
	"cosmossdk.io/log"
	authkeeper "cosmossdk.io/x/auth/keeper"
	bankkeeper "cosmossdk.io/x/bank/keeper"
	banktestutil "cosmossdk.io/x/bank/testutil"
	"cosmossdk.io/x/nft"
	nftkeeper "cosmossdk.io/x/nft/keeper"
	"cosmossdk.io/x/nft/simulation"
	"cosmossdk.io/x/nft/testutil"
	stakingkeeper "cosmossdk.io/x/staking/keeper"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/runtime"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	simtypes "github.com/cosmos/cosmos-sdk/types/simulation"
)

type SimTestSuite struct {
	suite.Suite

	ctx sdk.Context

	app               *runtime.App
	codec             codec.Codec
	interfaceRegistry codectypes.InterfaceRegistry
	txConfig          client.TxConfig
	accountKeeper     authkeeper.AccountKeeper
	bankKeeper        bankkeeper.Keeper
	stakingKeeper     *stakingkeeper.Keeper
	nftKeeper         nftkeeper.Keeper
}

func (suite *SimTestSuite) SetupTest() {
	app, err := simtestutil.Setup(
		depinject.Configs(
			testutil.AppConfig,
			depinject.Supply(log.NewNopLogger()),
		),
		&suite.codec,
		&suite.interfaceRegistry,
		&suite.txConfig,
		&suite.accountKeeper,
		&suite.bankKeeper,
		&suite.stakingKeeper,
		&suite.nftKeeper,
	)
	suite.Require().NoError(err)

	suite.app = app
	suite.ctx = app.BaseApp.NewContext(false)
}

func (suite *SimTestSuite) TestWeightedOperations() {
	weightedOps := simulation.WeightedOperations(
		suite.interfaceRegistry,
		make(simtypes.AppParams),
		suite.codec,
		suite.txConfig,
		suite.accountKeeper,
		suite.bankKeeper,
		suite.nftKeeper,
	)

	// setup 3 accounts
	s := rand.NewSource(1)
	r := rand.New(s)
	accs := suite.getTestingAccounts(r, 3)

	expected := []struct {
		weight     int
		opMsgRoute string
		opMsgName  string
	}{
		{simulation.WeightSend, nft.ModuleName, simulation.TypeMsgSend},
	}

	for i, w := range weightedOps {
		operationMsg, _, err := w.Op()(r, suite.app.BaseApp, suite.ctx, accs, "")
		suite.Require().NoError(err)

		// the following checks are very much dependent from the ordering of the output given
		// by WeightedOperations. if the ordering in WeightedOperations changes some tests
		// will fail
		suite.Require().Equal(expected[i].weight, w.Weight(), "weight should be the same")
		suite.Require().Equal(expected[i].opMsgRoute, operationMsg.Route, "route should be the same")
		suite.Require().Equal(expected[i].opMsgName, operationMsg.Name, "operation Msg name should be the same")
	}
}

func (suite *SimTestSuite) getTestingAccounts(r *rand.Rand, n int) []simtypes.Account {
	accounts := simtypes.RandomAccounts(r, n)

	initAmt := suite.stakingKeeper.TokensFromConsensusPower(suite.ctx, 200000)
	initCoins := sdk.NewCoins(sdk.NewCoin("stake", initAmt))

	// add coins to the accounts
	for _, account := range accounts {
		acc := suite.accountKeeper.NewAccountWithAddress(suite.ctx, account.Address)
		suite.accountKeeper.SetAccount(suite.ctx, acc)
		suite.Require().NoError(banktestutil.FundAccount(suite.ctx, suite.bankKeeper, account.Address, initCoins))
	}

	return accounts
}

func (suite *SimTestSuite) TestSimulateMsgSend() {
	s := rand.NewSource(1)
	r := rand.New(s)
	accounts := suite.getTestingAccounts(r, 2)
	blockTime := time.Now().UTC()
	ctx := suite.ctx.WithHeaderInfo(header.Info{Time: blockTime})

	// execute operation
	registry := suite.interfaceRegistry
	op := simulation.SimulateMsgSend(codec.NewProtoCodec(registry), suite.txConfig, suite.accountKeeper, suite.bankKeeper, suite.nftKeeper)
	operationMsg, futureOperations, err := op(r, suite.app.BaseApp, ctx, accounts, "")
	suite.Require().NoError(err)

	var msg nft.MsgSend
	err = proto.Unmarshal(operationMsg.Msg, &msg)
	suite.Require().NoError(err)
	suite.Require().True(operationMsg.OK)
	suite.Require().Len(futureOperations, 0)
}

func TestSimTestSuite(t *testing.T) {
	suite.Run(t, new(SimTestSuite))
}
