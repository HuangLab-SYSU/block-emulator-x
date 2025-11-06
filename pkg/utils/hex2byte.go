package utils

import (
	"encoding/hex"
	"strings"
)

func Hex2Bytes(s string) []byte {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}

	b, _ := hex.DecodeString(s)

	return b
}
