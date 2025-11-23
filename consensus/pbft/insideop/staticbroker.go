package insideop

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"log/slog"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/chain"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type StaticBrokerInsideOp struct {
	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.

	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewStaticBrokerInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper, chain *chain.Chain, txPool txpool.TxPool, cfg config.ConsensusNodeCfg, lp config.LocalParams) *StaticBrokerInsideOp {
	return &StaticBrokerInsideOp{
		conn:     conn,
		resolver: resolver,
		chain:    chain,
		txPool:   txPool,
		cfg:      cfg,
		lp:       lp,
	}
}

func (s *StaticBrokerInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := s.txPool.PackTxs()
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	b, err := s.chain.GenerateBlock(ctx, s.lp.WalletAddr, txs)
	if err != nil {
		return nil, fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	p, err := WrapProposal(b)
	if err != nil {
		return nil, fmt.Errorf("WrapProposal failed: %w", err)
	}

	slog.InfoContext(ctx, "block is generated in static broker module", "shard ID", s.chain.GetShardID(), "block height", b.Header.Number, "block create time", b.Header.CreateTime)

	return p, nil
}

func (s *StaticBrokerInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
	if proposal.ProposalType != message.BlockProposalType {
		return fmt.Errorf("invalid proposal type")
	}

	var b *block.Block
	if err := gob.NewDecoder(bytes.NewReader(proposal.Payload)).Decode(&b); err != nil {
		return fmt.Errorf("invalid payload, decode failed: %w", err)
	}

	if err := s.chain.ValidateBlock(ctx, b); err != nil {
		return fmt.Errorf("validate block failed: %w", err)
	}

	return nil
}

// ProposalCommitAndDeliver of StaticBrokerInsideOp contains:
// 1. apply the proposal to the chain.
// 2. send blockInfoMsg to the supervisor.
func (s *StaticBrokerInsideOp) ProposalCommitAndDeliver(ctx context.Context, proposal *message.Proposal) error {
	switch proposal.ProposalType {
	case message.BlockProposalType:
		if err := s.blockProposalCommitAndDeliver(ctx, proposal); err != nil {
			return fmt.Errorf("deliver the confirmed block proposal failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid proposal type = %s", proposal.ProposalType)
	}

	return nil
}

func (s *StaticBrokerInsideOp) Close() {}

func (s *StaticBrokerInsideOp) blockProposalCommitAndDeliver(ctx context.Context, proposal *message.Proposal) error {
	var b block.Block
	if err := gob.NewDecoder(bytes.NewReader(proposal.Payload)).Decode(&b); err != nil {
		return fmt.Errorf("invalid payload, decode as block failed: %w", err)
	}
	// commit block - add block to the blockchain
	if err := s.chain.AddBlock(ctx, &b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	// deliver this block info to the supervisor
	innerTxs, b1Txs, b2Txs := s.splitTxs(ctx, b.Body.TxList)
	if err := s.deliverBlockInfo2Supervisor(ctx, innerTxs, b1Txs, b2Txs, b); err != nil {
		return fmt.Errorf("deliverBlockInfo2Supervisor failed: %w", err)
	}

	return nil
}

func (s *StaticBrokerInsideOp) splitTxs(ctx context.Context, txs []transaction.Transaction) ([]transaction.Transaction, []transaction.Transaction, []transaction.Transaction) {
	innerTxs, b1Txs, b2Txs := make([]transaction.Transaction, 0), make([]transaction.Transaction, 0), make([]transaction.Transaction, 0)

	for _, tx := range txs {
		switch tx.BrokerStage {
		case transaction.RawTxBrokerStage:
			innerTxs = append(innerTxs, tx)
		case transaction.Sigma1BrokerStage:
			b1Txs = append(b1Txs, tx)
		case transaction.Sigma2BrokerStage:
			b2Txs = append(b2Txs, tx)
		default:
			slog.ErrorContext(ctx, "split tx error, broker stage invalid", "broker stage", tx.BrokerStage)
		}
	}

	return innerTxs, b1Txs, b2Txs
}

func (s *StaticBrokerInsideOp) deliverBlockInfo2Supervisor(ctx context.Context, innerTxs, b1Txs, b2Txs []transaction.Transaction, b block.Block) error {
	bbm := &message.BrokerBlockInfoMsg{
		InnerShardTxs:    innerTxs,
		Broker1Txs:       b1Txs,
		Broker2Txs:       b2Txs,
		Epoch:            s.chain.GetEpochID(),
		ShardID:          s.chain.GetShardID(),
		BlockProposeTime: b.Header.CreateTime,
		BlockCommitTime:  time.Now(),
	}

	w, err := message.WrapMsg(bbm)
	if err != nil {
		return fmt.Errorf("WrapMsg failed: %w", err)
	}

	spv, err := s.resolver.GetSupervisor()
	if err != nil {
		return fmt.Errorf("GetSupervisor failed: %w", err)
	}

	go s.conn.SendMessage(ctx, spv, w)

	return nil
}
