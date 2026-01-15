package utils

import (
	"fmt"
	"math/big"

	"github.com/holiman/uint256"
)

func BigToUInt256(b *big.Int) (*uint256.Int, error) {
	if b.Sign() < 0 {
		return nil, fmt.Errorf("the input big int is a negative number")
	}

	u, overflow := uint256.FromBig(b)
	if overflow {
		return nil, fmt.Errorf("call FromBig overflow")
	}

	return u, nil
}
