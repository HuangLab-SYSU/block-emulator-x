package block

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoltDb(t *testing.T) {
	testDbDir := "bolt-storage-test"
	testDbFile := "bolt"
	// create dir
	err := os.MkdirAll(testDbDir, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
	err = os.WriteFile(filepath.Join(testDbDir, testDbFile), nil, os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}

	boltDb, err := NewBoltStore(filepath.Join(testDbDir, testDbFile))
	if err != nil {
		t.Fatal("newBoltStore failed, err: ", err)
	}
	RunComplianceTests(t, boltDb, boltDb.clear)
}

// clear is only used for the test.
func (b *BoltStore) clear() error {
	return os.RemoveAll("bolt-storage-test")
}
