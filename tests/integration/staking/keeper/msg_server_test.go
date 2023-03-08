package keeper_test

import (
	"testing"
	"time"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/assert"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil/configurator"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authkeeper "github.com/cosmos/cosmos-sdk/x/auth/keeper"
	bankkeeper "github.com/cosmos/cosmos-sdk/x/bank/keeper"
	banktestutil "github.com/cosmos/cosmos-sdk/x/bank/testutil"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	"github.com/cosmos/cosmos-sdk/x/staking/keeper"
	"github.com/cosmos/cosmos-sdk/x/staking/testutil"
	"github.com/cosmos/cosmos-sdk/x/staking/types"
)

func TestCancelUnbondingDelegation(t *testing.T) {
	// setup the app
	var (
		stakingKeeper *keeper.Keeper
		bankKeeper    bankkeeper.Keeper
		accountKeeper authkeeper.AccountKeeper
	)
	app, err := simtestutil.SetupWithConfiguration(
		configurator.NewAppConfig(
			configurator.BankModule(),
			configurator.TxModule(),
			configurator.StakingModule(),
			configurator.ParamsModule(),
			configurator.ConsensusModule(),
			configurator.AuthModule(),
		),
		simtestutil.DefaultStartUpConfig(),
		&stakingKeeper, &bankKeeper, &accountKeeper)
	assert.NilError(t, err)

	ctx := app.BaseApp.NewContext(false, cmtproto.Header{})
	msgServer := keeper.NewMsgServerImpl(stakingKeeper)
	bondDenom := stakingKeeper.BondDenom(ctx)

	// set the not bonded pool module account
	notBondedPool := stakingKeeper.GetNotBondedPool(ctx)
	startTokens := stakingKeeper.TokensFromConsensusPower(ctx, 5)

	assert.NilError(t, banktestutil.FundModuleAccount(bankKeeper, ctx, notBondedPool.GetName(), sdk.NewCoins(sdk.NewCoin(stakingKeeper.BondDenom(ctx), startTokens))))
	accountKeeper.SetModuleAccount(ctx, notBondedPool)

	moduleBalance := bankKeeper.GetBalance(ctx, notBondedPool.GetAddress(), stakingKeeper.BondDenom(ctx))
	assert.DeepEqual(t, sdk.NewInt64Coin(bondDenom, startTokens.Int64()), moduleBalance)

	// accounts
	delAddrs := simtestutil.AddTestAddrsIncremental(bankKeeper, stakingKeeper, ctx, 2, sdk.NewInt(10000))
	validators := stakingKeeper.GetValidators(ctx, 10)
	assert.Equal(t, len(validators), 1)

	validatorAddr, err := sdk.ValAddressFromBech32(validators[0].OperatorAddress)
	assert.NilError(t, err)
	delegatorAddr := delAddrs[0]

	// setting the ubd entry
	unbondingAmount := sdk.NewInt64Coin(stakingKeeper.BondDenom(ctx), 5)
	ubd := types.NewUnbondingDelegation(
		delegatorAddr, validatorAddr, 10,
		ctx.BlockTime().Add(time.Minute*10),
		unbondingAmount.Amount,
		0,
	)

	// set and retrieve a record
	stakingKeeper.SetUnbondingDelegation(ctx, ubd)
	resUnbond, found := stakingKeeper.GetUnbondingDelegation(ctx, delegatorAddr, validatorAddr)
	assert.Assert(t, found)
	assert.DeepEqual(t, ubd, resUnbond)

	testCases := []struct {
		Name      string
		ExceptErr bool
		req       types.MsgCancelUnbondingDelegation
		expErrMsg string
	}{
		{
			Name:      "invalid height",
			ExceptErr: true,
			req: types.MsgCancelUnbondingDelegation{
				DelegatorAddress: resUnbond.DelegatorAddress,
				ValidatorAddress: resUnbond.ValidatorAddress,
				Amount:           sdk.NewCoin(stakingKeeper.BondDenom(ctx), sdk.NewInt(4)),
				CreationHeight:   0,
			},
			expErrMsg: "unbonding delegation entry is not found at block height",
		},
		{
			Name:      "invalid coin",
			ExceptErr: true,
			req: types.MsgCancelUnbondingDelegation{
				DelegatorAddress: resUnbond.DelegatorAddress,
				ValidatorAddress: resUnbond.ValidatorAddress,
				Amount:           sdk.NewCoin("dump_coin", sdk.NewInt(4)),
				CreationHeight:   0,
			},
			expErrMsg: "invalid coin denomination",
		},
		{
			Name:      "validator not exists",
			ExceptErr: true,
			req: types.MsgCancelUnbondingDelegation{
				DelegatorAddress: resUnbond.DelegatorAddress,
				ValidatorAddress: sdk.ValAddress(sdk.AccAddress("asdsad")).String(),
				Amount:           unbondingAmount,
				CreationHeight:   0,
			},
			expErrMsg: "validator does not exist",
		},
		{
			Name:      "invalid delegator address",
			ExceptErr: true,
			req: types.MsgCancelUnbondingDelegation{
				DelegatorAddress: "invalid_delegator_addrtess",
				ValidatorAddress: resUnbond.ValidatorAddress,
				Amount:           unbondingAmount,
				CreationHeight:   0,
			},
			expErrMsg: "decoding bech32 failed",
		},
		{
			Name:      "invalid amount",
			ExceptErr: true,
			req: types.MsgCancelUnbondingDelegation{
				DelegatorAddress: resUnbond.DelegatorAddress,
				ValidatorAddress: resUnbond.ValidatorAddress,
				Amount:           unbondingAmount.Add(sdk.NewInt64Coin(bondDenom, 10)),
				CreationHeight:   10,
			},
			expErrMsg: "amount is greater than the unbonding delegation entry balance",
		},
		{
			Name:      "success",
			ExceptErr: false,
			req: types.MsgCancelUnbondingDelegation{
				DelegatorAddress: resUnbond.DelegatorAddress,
				ValidatorAddress: resUnbond.ValidatorAddress,
				Amount:           unbondingAmount.Sub(sdk.NewInt64Coin(bondDenom, 1)),
				CreationHeight:   10,
			},
		},
		{
			Name:      "success",
			ExceptErr: false,
			req: types.MsgCancelUnbondingDelegation{
				DelegatorAddress: resUnbond.DelegatorAddress,
				ValidatorAddress: resUnbond.ValidatorAddress,
				Amount:           unbondingAmount.Sub(unbondingAmount.Sub(sdk.NewInt64Coin(bondDenom, 1))),
				CreationHeight:   10,
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			_, err := msgServer.CancelUnbondingDelegation(ctx, &testCase.req)
			if testCase.ExceptErr {
				assert.ErrorContains(t, err, testCase.expErrMsg)
			} else {
				assert.NilError(t, err)
				balanceForNotBondedPool := bankKeeper.GetBalance(ctx, sdk.AccAddress(notBondedPool.GetAddress()), bondDenom)
				assert.DeepEqual(t, balanceForNotBondedPool, moduleBalance.Sub(testCase.req.Amount))
				moduleBalance = moduleBalance.Sub(testCase.req.Amount)
			}
		})
	}
}

func TestRotateConsPubKey(t *testing.T) {
	// setup the app
	var (
		stakingKeeper *keeper.Keeper
		bankKeeper    bankkeeper.Keeper
		accountKeeper authkeeper.AccountKeeper
	)
	app, err := simtestutil.SetupWithConfiguration(
		configurator.NewAppConfig(
			configurator.BankModule(),
			configurator.TxModule(),
			configurator.StakingModule(),
			configurator.ParamsModule(),
			configurator.ConsensusModule(),
			configurator.AuthModule(),
		),
		simtestutil.DefaultStartUpConfig(),
		&accountKeeper, &bankKeeper, &stakingKeeper)
	assert.NilError(t, err)

	ctx := app.BaseApp.NewContext(false, cmtproto.Header{})
	msgServer := keeper.NewMsgServerImpl(stakingKeeper)
	bondDenom := stakingKeeper.BondDenom(ctx)

	addrs := simtestutil.AddTestAddrsIncremental(bankKeeper, stakingKeeper, ctx, 5, stakingKeeper.TokensFromConsensusPower(ctx, 300))
	valAddrs := simtestutil.ConvertAddrsToValAddrs(addrs)
	pks := []cryptotypes.PubKey{PKs[0], PKs[499]}

	val1 := testutil.NewValidator(t, valAddrs[0], pks[0])
	stakingKeeper.SetValidator(ctx, val1)
	stakingKeeper.SetValidatorByConsAddr(ctx, val1)
	stakingKeeper.SetNewValidatorByPowerIndex(ctx, val1)

	testCases := []struct {
		Name          string
		Pass          bool
		sender        sdk.AccAddress
		validator     sdk.ValAddress
		newPubKey     cryptotypes.PubKey
		rotationLimit uint64
	}{
		{
			Name:          "not existing validator check",
			sender:        addrs[1],
			validator:     valAddrs[1],
			newPubKey:     pks[1],
			rotationLimit: 10,
			Pass:          false,
		},
		{
			Name:          "consensus pubkey rotation limit check",
			sender:        addrs[0],
			validator:     val1.GetOperator(),
			newPubKey:     pks[1],
			rotationLimit: 0,
			Pass:          false,
		},
		{
			Name:          "successful consensus pubkey rotation",
			sender:        addrs[0],
			validator:     val1.GetOperator(),
			newPubKey:     pks[1],
			rotationLimit: 10,
			Pass:          true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.Name, func(t *testing.T) {
			params := stakingKeeper.GetParams(ctx)
			params.ConsPubkeyRotationFee = sdk.NewInt64Coin(bondDenom, 1000)
			params.MaxConsPubkeyRotations = testCase.rotationLimit
			err := stakingKeeper.SetParams(ctx, params)
			require.NoError(t, err)

			oldDistrBalance := bankKeeper.GetBalance(ctx, accountKeeper.GetModuleAddress(distrtypes.ModuleName), bondDenom)
			msg, err := types.NewMsgRotateConsPubKey(
				sdk.ValAddress(testCase.sender),
				testCase.newPubKey,
			)
			require.NoError(t, err)

			_, err = msgServer.RotateConsPubKey(ctx, msg)

			if testCase.Pass {

				require.NoError(t, err)

				// rotation fee payment from sender to distrtypes
				newDistrBalance := bankKeeper.GetBalance(ctx, accountKeeper.GetModuleAddress(distrtypes.ModuleName), bondDenom)
				require.Equal(t, newDistrBalance, oldDistrBalance.Add(params.ConsPubkeyRotationFee))

				// validator consensus pubkey update check
				validator, found := stakingKeeper.GetValidator(ctx, testCase.validator)
				require.True(t, found)

				consAddr, err := validator.GetConsAddr()
				require.NoError(t, err)
				require.Equal(t, consAddr.String(), sdk.ConsAddress(testCase.newPubKey.Address()).String())

				// consensus rotation history set check
				historyObjects := stakingKeeper.GetValidatorConsPubKeyRotationHistory(ctx, testCase.validator)
				require.Len(t, historyObjects, 1)
				historyObjects = stakingKeeper.GetBlockConsPubKeyRotationHistory(ctx, ctx.BlockHeight())
				require.Len(t, historyObjects, 1)
			} else {
				require.Error(t, err)
			}
		})
	}
}
