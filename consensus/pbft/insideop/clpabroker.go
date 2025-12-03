package insideop

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/insideop/txblockop"
	"github.com/HuangLab-SYSU/block-emulator/consensus/pbft/migration"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type CLPABrokerInsideOp struct {
	amm *migration.AccMigrateMetadata

	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.

	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	*txblockop.BrokerTxBlockOp

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewCLPABrokerInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper,
	chain *chain.Chain, txPool txpool.TxPool, amm *migration.AccMigrateMetadata,
	cfg config.ConsensusNodeCfg, lp config.LocalParams,
) (*CLPABrokerInsideOp, error) {
	bto, err := txblockop.NewBrokerTxBlockOp(chain, conn, resolver, cfg, lp)
	if err != nil {
		return nil, fmt.Errorf("newBlockCSVWriter failed: %w", err)
	}

	return &CLPABrokerInsideOp{
		amm:             amm,
		conn:            conn,
		resolver:        resolver,
		chain:           chain,
		txPool:          txPool,
		BrokerTxBlockOp: bto,
		cfg:             cfg,
		lp:              lp,
	}, nil
}

func (c *CLPABrokerInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	// TODO implement me
	panic("implement me")
}

func (c *CLPABrokerInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	if proposal.ProposalType != message.BlockProposalType && proposal.ProposalType != message.PartitionProposalType {
		return fmt.Errorf("invalid proposal type")
	}

	b, err := block.DecodeBlock(proposal.Payload)
	if err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	if err = c.chain.ValidateBlock(ctx, b); err != nil {
		return fmt.Errorf("validate block failed: %w", err)
	}

	return nil
}

func (c *CLPABrokerInsideOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	// TODO implement me
	panic("implement me")
}

func (c *CLPABrokerInsideOp) Close() {
	_ = c.BrokerTxBlockOp.Close()
	_ = c.chain.Close()
}
