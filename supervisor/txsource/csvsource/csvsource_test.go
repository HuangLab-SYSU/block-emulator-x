package csvsource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testdataPath = "testdata.csv"

func TestCSVSource_ReadTxs(t *testing.T) {
	cs, err := NewCSVSource(testdataPath)
	require.NoError(t, err)

	txs, err := cs.ReadTxs(10)
	require.NoError(t, err)
	// only 5 data in this file
	require.Len(t, txs, 5)
}
