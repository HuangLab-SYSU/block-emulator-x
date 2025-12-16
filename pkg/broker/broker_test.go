package broker

import (
	"math/big"
	"testing"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
	"github.com/stretchr/testify/require"
)

const (
	senderAddrStr   = "0x12312312738123"
	receiverAddrStr = "0xabcd7819279123"
)

func TestBroker(t *testing.T) {
	cfg := config.BrokerModuleCfg{
		BrokerFilePath: "./broker",
		BrokerNum:      10,
	}
	bm, err := NewBrokerManager(cfg)
	require.NoError(t, err)

	senderAddr, err := utils.Hex2Addr(senderAddrStr)
	require.NoError(t, err)
	receiverAddr, err := utils.Hex2Addr(receiverAddrStr)
	require.NoError(t, err)

	tx := transaction.Transaction{
		Sender:     senderAddr,
		Recipient:  receiverAddr,
		Value:      big.NewInt(100),
		Nonce:      100,
		CreateTime: time.Now(),
	}
	_, err = bm.CreateRawTxsRandomBroker([]transaction.Transaction{tx})
	require.NoError(t, err)

	b1txs, b2txs := bm.CreateBrokerTxs()
	require.Len(t, b1txs, 1)
	require.Len(t, b2txs, 0)

	err = bm.ConfirmBrokerTx(b1txs[0])
	require.NoError(t, err)

	b1txs, b2txs = bm.CreateBrokerTxs()
	require.Len(t, b1txs, 0)
	require.Len(t, b2txs, 1)

	err = bm.ConfirmBrokerTx(b2txs[0])
	require.NoError(t, err)
}
