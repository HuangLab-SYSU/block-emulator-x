package outsideop

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
)

// CLPALocOutsideOp is a ShardOutsideMsgHandler for the dynamic-sharding consensus (e.g., CLPA).
type CLPALocOutsideOp struct {
	amm    *migration.AccMigrateMetadata
	txPool txpool.TxPool
}

func NewCLPALocOutsideOp(txPool txpool.TxPool, amm *migration.AccMigrateMetadata) *CLPALocOutsideOp {
	return &CLPALocOutsideOp{amm: amm, txPool: txPool}
}

func (c *CLPALocOutsideOp) HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	switch msg.GetMsgType() {
	case message.ReceiveTxsMessageType:
		var rt message.ReceiveTxsMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&rt); err != nil {
			return fmt.Errorf("decode ReceiveTxs msg failed: %w", err)
		}

		if err := c.txPool.AddTxs(rt.Txs); err != nil {
			return fmt.Errorf("tx pool add txs: %w", err)
		}

		slog.InfoContext(ctx, "received txs are added in the tx pool successfully", "tx size", len(rt.Txs))

	case message.CLPARepartitionStartMessageType:
		var cr message.CLPARepartitionStartMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&cr); err != nil {
			return fmt.Errorf("decode CLPARepartitionStart msg failed: %w", err)
		}

		if err := c.amm.UpdateByRepartitionStartMsg(&cr); err != nil {
			return fmt.Errorf("set account migration metadata by RepartitionStartMsg failed: %w", err)
		}

		slog.InfoContext(ctx, "handle CLPARepartitionStartMsg successfully")

	case message.AccountMigrationMessageType:
		var aat message.AccountMigrationMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&aat); err != nil {
			return fmt.Errorf("decode AccountMigrationMsg failed: %w", err)
		}

		if err := c.amm.CollectStatesByMsg(&aat); err != nil {
			return fmt.Errorf("collect states by AccountMigrationMsg failed: %w", err)
		}

		slog.InfoContext(ctx, "handle AccountMigrationMsg successfully")

	default:
		return fmt.Errorf("unknown msg type: %s", msg.GetMsgType())
	}

	return nil
}
