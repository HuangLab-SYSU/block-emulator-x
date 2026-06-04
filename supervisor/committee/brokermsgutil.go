package committee

import "github.com/HuangLab-SYSU/block-emulator-x/pkg/message"

// brokerBlockTxCount returns the number of transactions reported in a broker block info message.
func brokerBlockTxCount(b *message.BrokerBlockInfoMsg) int {
	return len(b.InnerShardTxs) + len(b.Broker1Txs) + len(b.Broker2Txs) +
		len(b.Relay1Txs) + len(b.Relay2Txs)
}
