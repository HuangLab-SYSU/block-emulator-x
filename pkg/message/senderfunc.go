package message

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

func SendWrappedTxs2Shards(
	ctx context.Context,
	txs [][]transaction.Transaction,
	conn *network.ConnHandler,
	r nodetopo.NodeMapper,
) error {
	node2Msg := make(map[nodetopo.NodeInfo]*rpcserver.WrappedMsg, len(txs))

	// pack messages and send them
	for i, txs2Shard := range txs {
		if len(txs2Shard) == 0 {
			continue
		}

		l, err := r.GetLeader(int64(i))
		if err != nil {
			return fmt.Errorf("GetLeader failed: %w", err)
		}

		w, err := WrapMsg(&ReceiveTxsMsg{Txs: txs2Shard})
		if err != nil {
			return fmt.Errorf("WrapMsg failed: %w", err)
		}

		node2Msg[l] = w
	}

	conn.MSendDifferentMessages(ctx, node2Msg)

	return nil
}
