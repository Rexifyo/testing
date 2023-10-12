package types

import "cosmossdk.io/collections"

const (
	// ModuleName is the module name constant used in many places
	ModuleName = "protocolpool"

	// StoreKey is the store key string for protocolpool
	StoreKey = ModuleName

	// RouterKey is the message route for protocolpool
	RouterKey = ModuleName

	// GovModuleName is the name of the gov module
	GovModuleName = "gov"
)

var (
	BudgetKey    = collections.NewPrefix(0)
	DistrInfoKey = collections.NewPrefix(1)
)
