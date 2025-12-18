package utils

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
)

func Hex2Addr(s string) (account.Address, error) {
	var addr account.Address

	b, err := Hex2Bytes(s)
	if err != nil {
		return addr, fmt.Errorf("hex2byte err: %w", err)
	}

	copy(addr[:], b)

	return addr, nil
}

func Hex2Bytes(s string) ([]byte, error) {
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		s = s[2:]
	}

	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("failed to decoding hex string: %w", err)
	}

	return b, nil
}
