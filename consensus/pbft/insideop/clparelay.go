package insideop

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type CLPARelayInsideOp struct {
	amm *migration.AccMigrateMetadata

	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.

	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewCLPARelayInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper,
	chain *chain.Chain, txPool txpool.TxPool, amm *migration.AccMigrateMetadata,
	cfg config.ConsensusNodeCfg, lp config.LocalParams,
) *CLPARelayInsideOp {
	return &CLPARelayInsideOp{
		amm:      amm,
		conn:     conn,
		resolver: resolver,
		chain:    chain,
		txPool:   txPool,
		cfg:      cfg,
		lp:       lp,
	}
}

func (C *CLPARelayInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// TODO implement me
	panic("implement me")
}

func (C *CLPARelayInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (C *CLPARelayInsideOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (C *CLPARelayInsideOp) Close() {
	// TODO implement me
	panic("implement me")
}
