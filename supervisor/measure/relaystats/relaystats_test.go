package relaystats

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/supervisor/txsource/randomsource"
)

const (
	txSize = 10
)

func TestRelayStats_UpdateMeasureRecord(t *testing.T) {
	_ = os.RemoveAll("test_dir")
	r, err := NewRelayStats("test_dir/")

	t.Cleanup(func() {
		if err = os.RemoveAll("test_dir"); err != nil {
			t.Fatalf("failed to remove test directory: %v", err)
		}
	})

	require.NoError(t, err)
	err = r.UpdateMeasureRecord(initInputMsg(t))
	require.NoError(t, err)
	err = r.OutputResultAndClose()
	require.NoError(t, err)
}

func initInputMsg(t *testing.T) *rpcserver.WrappedMsg {
	txSource := randomsource.NewRandomSource()
	innerShardTxs, _ := txSource.ReadTxs(txSize)
	cTxs, _ := txSource.ReadTxs(txSize)
	r1Txs, r2Txs := make([]transaction.Transaction, txSize), make([]transaction.Transaction, txSize)
	for i, cTx := range cTxs {
		txHash, err := cTx.Hash()
		require.NoError(t, err)
		r1 := transaction.NewTransaction(cTx.Sender, cTx.Recipient, cTx.Value, cTx.Nonce, cTx.CreateTime)
		r1.ROriginalHash = txHash
		r1.RelayStage = 1

		r2 := transaction.NewTransaction(cTx.Sender, cTx.Recipient, cTx.Value, cTx.Nonce, cTx.CreateTime)
		r2.ROriginalHash = txHash
		r2.RelayStage = 2

		r1Txs[i] = *r1
		r2Txs[i] = *r2
	}

	m := &message.RelayBlockInfoMsg{
		InnerShardTxs:    innerShardTxs,
		Relay1Txs:        r1Txs,
		Relay2Txs:        r2Txs,
		Epoch:            0,
		BlockProposeTime: time.Now(),
		BlockCommitTime:  time.Now().Add(time.Second),
	}

	msg, err := message.WrapMsg(m)
	require.NoError(t, err)
	return msg
}
