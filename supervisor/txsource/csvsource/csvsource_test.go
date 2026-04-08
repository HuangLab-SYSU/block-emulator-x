package csvsource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testdataPath = "testdata.csv"
const filterContractTxs = true

func TestCSVSource_ReadTxs(t *testing.T) {
	cs, err := NewCSVSource(testdataPath, filterContractTxs)
	require.NoError(t, err)

	txs, err := cs.ReadTxs(10)
	require.NoError(t, err)
	// only 5 data in this file
	require.Len(t, txs, 5)

	txs, err = cs.ReadTxs(10)
	require.Len(t, txs, 0)
}
