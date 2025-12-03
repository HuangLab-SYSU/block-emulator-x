package txblockop

import (
	"context"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type TxBlockOp interface {
	BuildTxBlockProposal(ctx context.Context, txs []transaction.Transaction) (*message.Proposal, error)
	BlockCommitAndDeliver(ctx context.Context, isLeader bool, b *block.Block) error
}

func NewTxBlockOp(conn *network.P2PConn, resolver nodetopo.NodeMapper, chain *chain.Chain, cfg config.ConsensusNodeCfg, lp config.LocalParams) (TxBlockOp, error) {
	var tbo TxBlockOp

	switch cfg.ConsensusType {
	case config.StaticRelayConsensus, config.CLPARelayConsensus:
		tbo = NewRelayTxBlockOp(chain, conn, resolver, cfg, lp)
	case config.StaticBrokerConsensus, config.CLPABrokerConsensus:
		tbo = NewBrokerTxBlockOp(chain, conn, resolver, cfg, lp)
	default:
		return nil, fmt.Errorf("unknown consensus type: %s", cfg.ConsensusType)
	}

	return tbo, nil
}
