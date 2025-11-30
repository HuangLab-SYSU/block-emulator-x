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
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/txpool"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type StaticRelayInsideOp struct {
	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.
	resolver nodetopo.NodeMapper // resolver gives the information of all consensus nodes and shards.

	chain  *chain.Chain  // chain is the data-structure of blockchain.
	txPool txpool.TxPool // txPool is the transactions pool.

	cfg config.ConsensusNodeCfg
	lp  config.LocalParams
}

func NewStaticRelayInsideOp(conn *network.P2PConn, resolver nodetopo.NodeMapper, chain *chain.Chain, txPool txpool.TxPool, cfg config.ConsensusNodeCfg, lp config.LocalParams) *StaticRelayInsideOp {
	return &StaticRelayInsideOp{
		conn:     conn,
		resolver: resolver,
		chain:    chain,
		txPool:   txPool,
		cfg:      cfg,
		lp:       lp,
	}
}

func (s *StaticRelayInsideOp) BuildProposal(ctx context.Context) (*message.Proposal, error) {
	txs, err := s.txPool.PackTxs(int(s.cfg.Limit))
	if err != nil {
		return nil, fmt.Errorf("txPool.PackTxs failed: %w", err)
	}

	// if a transaction is a cross-shard tx, modify its RelayOpt
	mTxs, err := modifyTxRelayOpt(ctx, txs, s.chain)
	if err != nil {
		return nil, fmt.Errorf("modifyTxRelayOpt failed: %w", err)
	}

	b, err := s.chain.GenerateBlock(ctx, s.lp.WalletAddr, mTxs)
	if err != nil {
		return nil, fmt.Errorf("chain.GenerateBlock failed: %w", err)
	}

	p, err := WrapProposal(b, message.BlockProposalType)
	if err != nil {
		return nil, fmt.Errorf("WrapProposal failed: %w", err)
	}

	slog.InfoContext(ctx, "block is generated in static relay module", "shard ID", s.chain.GetShardID(), "block height", b.Header.Number, "block create time", b.Header.CreateTime)

	return p, nil
}

func (s *StaticRelayInsideOp) ValidateProposal(ctx context.Context, proposal *message.Proposal) error {
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

// ProposalCommitAndDeliver of StaticRelayInsideOp contains:
// 1. apply the proposal to the chain.
// 2.1. send blockInfoMsg to the supervisor.
// 2.2. send relay-txs to leaders of other shards.
func (s *StaticRelayInsideOp) ProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	switch proposal.ProposalType {
	case message.BlockProposalType:
		if err := s.blockProposalCommitAndDeliver(ctx, isLeader, proposal); err != nil {
			return fmt.Errorf("deliver and commit the tx block proposal failed: %w", err)
		}
	default:
		return fmt.Errorf("invalid proposal type = %s", proposal.ProposalType)
	}

	return nil
}

func (s *StaticRelayInsideOp) Close() {}

func (s *StaticRelayInsideOp) blockProposalCommitAndDeliver(ctx context.Context, isLeader bool, proposal *message.Proposal) error {
	var b block.Block
	if err := gob.NewDecoder(bytes.NewReader(proposal.Payload)).Decode(&b); err != nil {
		return fmt.Errorf("invalid payload, decode as block failed: %w", err)
	}
	// commit block - add block to the blockchain
	if err := s.chain.AddBlock(ctx, &b); err != nil {
		return fmt.Errorf("chain.AddBlock failed: %w", err)
	}

	slog.Info("block is added in static relay module", "block height", b.Header.Number)

	// if this node is not a leader, skip
	if !isLeader {
		return nil
	}

	// deliver this block info to the supervisor
	innerTxs, r1Txs, r2Txs := s.splitTxs(ctx, b.TxList)

	if err := s.deliverBlockInfo2Supervisor(ctx, innerTxs, r1Txs, r2Txs, b); err != nil {
		return fmt.Errorf("deliverBlockInfo2Supervisor failed: %w", err)
	}

	accountLocations, err := getAccountLocationsInTxs(ctx, s.chain, b.TxList)
	if err != nil {
		return fmt.Errorf("getAccountLocationsInTxs failed: %w", err)
	}

	if err = s.sendRelayedTxs(ctx, r1Txs, accountLocations); err != nil {
		return fmt.Errorf("sendRelayedTxs failed: %w", err)
	}

	return nil
}

// splitTxs split transactions to inner-shard txs, relay1 txs and relay2 txs.
func (s *StaticRelayInsideOp) splitTxs(ctx context.Context, txs []transaction.Transaction) ([]transaction.Transaction, []transaction.Transaction, []transaction.Transaction) {
	innerTxs, r1txs, r2txs := make([]transaction.Transaction, 0), make([]transaction.Transaction, 0), make([]transaction.Transaction, 0)

	for _, tx := range txs {
		switch tx.RelayStage {
		case transaction.UndeterminedRelayTx:
			innerTxs = append(innerTxs, tx)
		case transaction.Relay1Tx:
			r1txs = append(r1txs, tx)
		case transaction.Relay2Tx:
			r2txs = append(r2txs, tx)
		default:
			slog.ErrorContext(ctx, "invalid relay tx stage", "relay stage", tx.RelayStage)
		}
	}

	return innerTxs, r1txs, r2txs
}

func (s *StaticRelayInsideOp) deliverBlockInfo2Supervisor(ctx context.Context, innerTxs, r1Txs, r2Txs []transaction.Transaction, b block.Block) error {
	rbm := &message.RelayBlockInfoMsg{
		InnerShardTxs:    innerTxs,
		Relay1Txs:        r1Txs,
		Relay2Txs:        r2Txs,
		ShardID:          s.chain.GetShardID(),
		Epoch:            s.chain.GetEpochID(),
		BlockProposeTime: b.Header.CreateTime,
		BlockCommitTime:  time.Now(),
	}

	w, err := message.WrapMsg(rbm)
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

func (s *StaticRelayInsideOp) sendRelayedTxs(ctx context.Context, r1Txs []transaction.Transaction, accountLocations map[account.Account]int64) error {
	// for relay1 txs, send relay messages to other shards.
	relayedTxs := make([][]transaction.Transaction, s.cfg.ShardNum)

	// split r1Txs into all shards
	for _, tx := range r1Txs {
		// the next destination of relay1 tx should be calculated according to the recipient addr.
		shardID, ok := accountLocations[tx.Recipient]
		if !ok {
			slog.ErrorContext(ctx, "tx.Recipient is not found in accountLocations", "recipient", tx.Recipient)
			continue
		}

		// modify relay tx's RelayOpt
		updatedRelayedTx := tx
		updatedRelayedTx.RelayStage = transaction.Relay2Tx
		relayedTxs[shardID] = append(relayedTxs[shardID], updatedRelayedTx)
	}

	node2Msg := make(map[nodetopo.NodeInfo]*rpcserver.WrappedMsg, s.cfg.ShardNum)

	// pack messages and send them
	for i, txs := range relayedTxs {
		if len(txs) == 0 {
			continue
		}

		l, err := s.resolver.GetLeader(int64(i))
		if err != nil {
			return fmt.Errorf("GetLeader failed: %w", err)
		}

		w, err := message.WrapMsg(&message.ReceiveTxsMsg{Txs: txs})
		if err != nil {
			return fmt.Errorf("WrapMsg failed: %w", err)
		}

		node2Msg[l] = w
	}

	s.conn.MSendDifferentMessages(ctx, node2Msg)

	return nil
}
