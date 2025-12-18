package outsideop

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
)

type StaticLocOutsideOp struct {
	txPool txpool.TxPool
}

func NewStaticLocOutsideOp(txPool txpool.TxPool) *StaticLocOutsideOp {
	return &StaticLocOutsideOp{txPool: txPool}
}

func (s *StaticLocOutsideOp) HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	switch msg.GetMsgType() {
	case message.ReceiveTxsMessageType:
		var rt message.ReceiveTxsMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&rt); err != nil {
			return fmt.Errorf("decode ReceiveTxs msg failed: %w", err)
		}

		if err := s.txPool.AddTxs(rt.Txs); err != nil {
			return fmt.Errorf("tx pool add txs: %w", err)
		}

		slog.InfoContext(ctx, "received txs are added in the tx pool successfully", "tx size", len(rt.Txs))

	default:
		return fmt.Errorf("unknown msg type: %s", msg.GetMsgType())
	}

	return nil
}
