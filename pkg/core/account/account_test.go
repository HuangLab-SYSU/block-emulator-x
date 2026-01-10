package account

import (
	"encoding/hex"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testAddrByte      = "1234567890abcdef"
	testOutOfRangeStr = "10000000000000000000000000000000000001"
	testAmount        = 100000
)

func TestAccountState(t *testing.T) {
	addrByte, err := hex.DecodeString(testAddrByte)
	require.NoError(t, err)
	var addr Address
	copy(addr[:], addrByte)
	s := NewState(addr, 0)

	amount := big.NewInt(testAmount)
	var amountOutOfRange big.Int
	amountOutOfRange.SetString(testOutOfRangeStr, 10)

	// Debit and Credit.
	err = s.Debit(amount)
	require.NoError(t, err)

	err = s.Debit(&amountOutOfRange)
	require.Error(t, err)

	s.Credit(amount)

	// Encode and Decode
	sByte, err := s.Encode()
	require.NoError(t, err)

	decodedS, err := DecodeState(sByte)
	require.NoError(t, err)
	require.Equal(t, s, decodedS)
}
