package migrationblockop

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/exp/maps"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/nodetopo"
)

// MigrationBlockOp describes the operations for account-migration blocks.
type MigrationBlockOp struct {
	amm      *migration.AccMigrateMetadata
	conn     *network.ConnHandler // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper  // resolver gives the information of all consensus nodes and shards.

	chain *chain.Chain // chain is the data-structure of blockchain.

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewMigrationBlockOp(
	conn *network.ConnHandler,
	resolver nodetopo.NodeMapper,
	chain *chain.Chain,
	amm *migration.AccMigrateMetadata,
	cfg config.ConsensusNodeCfg,
	lp config.LocalParams,
) *MigrationBlockOp {
	return &MigrationBlockOp{amm: amm, conn: conn, resolver: resolver, chain: chain, lp: lp, cfg: cfg}
}

// BuildMigrationProposal builds a proposal containing an account-migration block.
func (m *MigrationBlockOp) BuildMigrationProposal(ctx context.Context) (*message.Proposal, error) {
	// If the number of received account-migration messages is enough (equal to ShardNum),
	// the leader will pack the partition proposal in a block.
	if len(m.amm.MigratedAccountStates) != int(m.cfg.ShardNum) {
		slog.InfoContext(
			ctx,
			"not all MigratedAccountStates is collected, do not propose",
			"expect",
			int(m.cfg.ShardNum),
			"actual",
			len(m.amm.MigratedAccountStates),
		)

		return nil, nil
	}

	accounts, states, err := m.amm.GetMigratedAddrStates()
	if err != nil {
		return nil, fmt.Errorf("GetMigratedAddrStates failed: %w", err)
	}

	b, err := m.chain.GenerateBlock(
		ctx,
		m.lp.WalletAddr,
		block.MigrationBlockType,
		block.Body{},
		block.MigrationOpt{MigratedAddrs: accounts, MigratedStates: states},
	)
	if err != nil {
		return nil, fmt.Errorf("GenerateMigrationBlock failed: %w", err)
	}

	p := message.WrapProposal(b)

	slog.InfoContext(ctx, "migration block proposal is generated", "height", b.Number)

	return p, nil
}

// MigrateAccounts migrates accounts.
func (m *MigrationBlockOp) MigrateAccounts(ctx context.Context) error {
	accountsMigratedOut := maps.Keys(m.amm.CurModifiedMap)

	states, err := m.chain.GetAccountStates(ctx, accountsMigratedOut)
	if err != nil {
		return fmt.Errorf("GetAccountStates failed: %w", err)
	}

	atMsgList := make([]message.AccountMigrationMsg, m.cfg.ShardNum)

	// Init the AccountMigrationMsg list
	for i := range atMsgList {
		atMsgList[i] = message.AccountMigrationMsg{
			SrcShard:      m.lp.ShardID,
			DestShard:     int64(i),
			Epoch:         m.amm.Epoch,
			AccountStates: make(map[account.Address]*account.State),
		}
	}

	for i, acc := range accountsMigratedOut {
		// If this shard is in not this shard, skip it
		if states[i].ShardLocation != uint64(m.lp.ShardID) {
			continue
		}

		destShardID := int64(m.amm.CurModifiedMap[acc])
		if destShardID == m.lp.ShardID {
			slog.Warn("unexpected CLPA result: destShardID == srcShardID", "ShardID", m.lp.ShardID)
		}

		atMsgList[destShardID].AccountStates[acc] = states[i]
	}

	sendMsgMap := make(map[nodetopo.NodeInfo]*rpcserver.WrappedMsg, len(atMsgList))

	for i := range atMsgList {
		l, err := m.resolver.GetLeader(int64(i))
		if err != nil {
			return fmt.Errorf("GetLeader failed: %w", err)
		}

		w, err := message.WrapMsg(&atMsgList[i])
		if err != nil {
			return fmt.Errorf("WrapMsg failed: %w", err)
		}

		sendMsgMap[l] = w
	}

	m.conn.MSendDifferentMessages(ctx, sendMsgMap)

	return nil
}

// MigrationBlockCommit commits the migration block and resets the status of the migration metadata.
func (m *MigrationBlockOp) MigrationBlockCommit(ctx context.Context, b *block.Block) error {
	// commit block - add block to the blockchain
	if err := m.chain.AddBlock(ctx, b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	m.chain.UpdateEpoch(m.amm.Epoch)
	m.amm.MigrationStatusReset()
	slog.Info("migration block is added", "block height", b.Number)

	return nil
}
