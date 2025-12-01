package randomsource

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewRandomSource(t *testing.T) {
	rs := NewRandomSource()
	txs, _ := rs.ReadTxs(100)
	require.Len(t, txs, 100)
	for i, tx := range txs {
		valueInt64 := tx.Value.Int64()
		require.Less(t, valueInt64, int64(upperBoundTransferAmount+1))
		require.Equal(t, tx.Nonce, uint64(i+1))
	}
}
