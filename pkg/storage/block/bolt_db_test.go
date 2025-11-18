package block

import (
	"os"
	"testing"

	"github.com/HuangLab-SYSU/block-emulator/config"
)

func TestBoltDb(t *testing.T) {
	testDbDir := "bolt-storage-test"
	// create dir
	err := os.MkdirAll(testDbDir, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	boltDb, err := NewBoltStore(config.BoltCfg{FilePathDir: testDbDir}, config.LocalParams{})
	if err != nil {
		t.Fatal("newBoltStore failed, err: ", err)
	}
	RunComplianceTests(t, boltDb, boltDb.clear)
}

// clear is only used for the test.
func (b *BoltStore) clear() error {
	return os.RemoveAll("bolt-storage-test")
}
