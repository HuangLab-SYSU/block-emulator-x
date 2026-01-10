package block

import (
	"os"
	"testing"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/stretchr/testify/require"
)

func TestBoltDb(t *testing.T) {
	testDbDir := "bolt-storage-test"
	// create dir
	err := os.MkdirAll(testDbDir, os.ModePerm)
	require.NoError(t, err)

	boltDb, err := NewBoltStore(config.BoltCfg{FilePathDir: testDbDir}, config.LocalParams{})
	require.NoError(t, err)
	RunComplianceTests(t, boltDb, boltDb.clear)
}

// clear is only used for the test.
func (b *BoltStore) clear() error {
	return os.RemoveAll("bolt-storage-test")
}
