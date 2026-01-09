package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestConvertTime2Str(t *testing.T) {
	require.NotEmpty(t, ConvertTime2Str(time.Now()))
	require.Empty(t, ConvertTime2Str(time.Time{}))
}

func TestHex2Addr(t *testing.T) {
	const (
		testHexWithPrefix    = "0x123456"
		testHexWithoutPrefix = "123456"
		badHex               = "0xabcdefg"
	)

	ans1, err := Hex2Addr(testHexWithPrefix)
	require.NoError(t, err)
	ans2, err := Hex2Addr(testHexWithoutPrefix)
	require.NoError(t, err)
	require.Equal(t, ans1, ans2)

	_, err = Hex2Addr(badHex)
	require.Error(t, err)
}
