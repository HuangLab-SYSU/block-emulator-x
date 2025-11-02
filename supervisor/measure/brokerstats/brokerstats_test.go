package brokerstats

import (
	"testing"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource/randomsource"
	"github.com/stretchr/testify/require"
)

const (
	txSize     = 10
	brokerAddr = "broker-account"
)

func TestBrokerStats_UpdateMeasureRecord(t *testing.T) {
	b := NewBrokerStats()
	err := b.UpdateMeasureRecord(initInputMsg(t))
	require.NoError(t, err)
}

func initInputMsg(t *testing.T) *rpcserver.WrappedMsg {
	txSource := randomsource.NewRandomSource()
	innerShardTxs, _ := txSource.ReadTxs(txSize)
	cTxs, _ := txSource.ReadTxs(txSize)
	b1Txs, b2Txs := make([]transaction.Transaction, txSize), make([]transaction.Transaction, txSize)
	for i, cTx := range cTxs {
		txHash, err := utils.CalcHash(&cTx)
		require.NoError(t, err)
		b1 := transaction.NewTransaction(cTx.Sender, cTx.Recipient, cTx.Value, cTx.Nonce, cTx.CreateTime)
		b1.BOriginalHash = txHash
		b1.BrokerStage = 1

		b2 := transaction.NewTransaction(cTx.Sender, cTx.Recipient, cTx.Value, cTx.Nonce, cTx.CreateTime)
		b2.BOriginalHash = txHash
		b2.BrokerStage = 2

		b1Txs[i] = *b1
		b2Txs[i] = *b2
	}

	m := &message.BrokerBlockInfoMsg{
		InnerShardTxs:    innerShardTxs,
		Broker1Txs:       b1Txs,
		Broker2Txs:       b2Txs,
		Epoch:            0,
		BlockProposeTime: time.Now(),
		BlockCommitTime:  time.Now().Add(time.Second),
	}

	msg, err := message.WrapMsg(m)
	require.NoError(t, err)
	return msg
}
