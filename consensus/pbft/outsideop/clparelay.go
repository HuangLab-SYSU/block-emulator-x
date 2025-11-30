package outsideop

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
)

type CLPARelayOutsideOp struct {
	amm    *migration.AccMigrateMetadata
	txPool txpool.TxPool
}

func NewCLPARelayOutsideOp(txp txpool.TxPool, amm *migration.AccMigrateMetadata) *CLPARelayOutsideOp {
	return &CLPARelayOutsideOp{
		amm:    amm,
		txPool: txp,
	}
}

func (c *CLPARelayOutsideOp) HandleMsgOutsideShard(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	switch msg.GetMsgType() {
	case message.ReceiveTxsMessageType:
		var rt message.ReceiveTxsMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&rt); err != nil {
			return fmt.Errorf("decode ReceiveTxs msg failed: %w", err)
		}

		if err := c.txPool.AddTxs(rt.Txs); err != nil {
			return fmt.Errorf("tx pool add txs: %w", err)
		}

		slog.InfoContext(ctx, "Received txs are added in the tx pool successfully", "tx size", len(rt.Txs))

	case message.CLPARepartitionStartMessageType:
		var cr message.CLPARepartitionStartMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&cr); err != nil {
			return fmt.Errorf("decode CLPARepartitionStart msg failed: %w", err)
		}

		if err := c.amm.UpdateByRepartitionStartMsg(&cr); err != nil {
			return fmt.Errorf("set account migration metadata by RepartitionStartMsg failed: %w", err)
		}

		slog.InfoContext(ctx, "handle CLPARepartitionStartMsg successfully")

	case message.AccountAndTxMigrationMessageType:
		var aat message.AccountAndTxMigrationMsg
		if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&aat); err != nil {
			return fmt.Errorf("decode AccountAndTxMigrationMsg failed: %w", err)
		}

		if err := c.txPool.AddTxs(aat.MigratedTxs); err != nil {
			return fmt.Errorf("tx pool add txs from AccountAndTxMigrationMsg failed: %w", err)
		}

		if err := c.amm.CollectStatesByMsg(&aat); err != nil {
			return fmt.Errorf("collect states by AccountAndTxMigrationMsg failed: %w", err)
		}

		slog.InfoContext(ctx, "handle AccountAndTxMigrationMsg successfully")

	default:
		return fmt.Errorf("unknown msg type: %s", msg.GetMsgType())
	}

	return nil
}

func (c *CLPARelayOutsideOp) Close() {
	// TODO implement me
	panic("implement me")
}
