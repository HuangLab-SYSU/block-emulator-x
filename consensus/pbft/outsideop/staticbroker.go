package outsideop

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

type StaticBrokerOutsideOp struct {
	txPool txpool.TxPool
}

func (s *StaticBrokerOutsideOp) HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	switch msg.GetMsgType() {
	case message.ReceiveTxsMessageType:
		var rt message.ReceiveTxsMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&rt); err != nil {
			return fmt.Errorf("decode ReceiveTxs msg failed: %v", err)
		}

		if err := s.txPool.AddTxs(rt.Txs); err != nil {
			return fmt.Errorf("tx pool add txs: %w", err)
		}

		slog.InfoContext(ctx, "Received txs are added in the tx pool successfully", "tx size", len(rt.Txs))

	default:
		return fmt.Errorf("unknown msg type: %s", msg.GetMsgType())
	}

	return nil
}

func (s *StaticBrokerOutsideOp) Close() {}
