package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

func getVMChainConfig(chainID int64) *params.ChainConfig {
	cc := *params.MainnetChainConfig
	cc.ChainID = big.NewInt(chainID)
	return &cc
}
