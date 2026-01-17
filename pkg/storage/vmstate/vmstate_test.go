package vmstate

import (
	"testing"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/stretchr/testify/require"
)

func TestNewVMStateStore(t *testing.T) {
	_, err := NewStateStore(config.StorageCfg{EthStorageCfg: config.EthStorageCfg{IsMemoryDB: true}}, config.LocalParams{})
	require.NoError(t, err)
}
