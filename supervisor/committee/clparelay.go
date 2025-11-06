package committee

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/pkg/partition"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource"
)

type CLPARelayCommittee struct {
	conn *network.P2PConn
	r    nodetopo.NodeMapper

	clpa *partition.CLPAState // clpa is the module of CLPA, aka., an account-reallocation algorithm.

	txSource    txsource.TxSource
	sl          stopLogic // sl is the logic of stop.
	unsentTxNum int64

	cfg config.SupervisorCfg
}

func NewCLPARelayCommittee(conn *network.P2PConn, r nodetopo.NodeMapper, cfg config.SupervisorCfg) (*CLPARelayCommittee, error) {
	ts, err := txsource.NewTxSource(cfg.TxSourceCfg)
	if err != nil {
		return nil, fmt.Errorf("NewTxSource failed: %w", err)
	}

	return &CLPARelayCommittee{
		conn:        conn,
		r:           r,
		clpa:        partition.NewCLPAState(clpaWeightPenalty, clpaMaxIterations, int(cfg.ShardNum)),
		txSource:    ts,
		sl:          stopLogic{stopThreshold: cfg.ShardNum * stopThresholdPerShard, stopCnt: 0},
		unsentTxNum: cfg.TxNumber,
		cfg:         cfg,
	}, nil
}

func (c *CLPARelayCommittee) SendTxsAndConsensus(ctx context.Context) error {
	// TODO implement me
	panic("implement me")
}

func (c *CLPARelayCommittee) HandleMsg(_ context.Context, msg *rpcserver.WrappedMsg) error {
	if msg.GetMsgType() != message.RelayBlockInfoMessageType {
		return fmt.Errorf("unexpected msg type: %s", msg.GetMsgType())
	}

	var bInfo message.RelayBlockInfoMsg
	if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&bInfo); err != nil {
		return fmt.Errorf("decode relayBlockInfoMsg: %w", err)
	}

	// update the stop module
	if len(bInfo.InnerShardTxs)+len(bInfo.Relay1Txs)+len(bInfo.Relay2Txs) == 0 {
		c.sl.stopCnt++
	} else {
		c.sl.stopCnt = 0 // reset 0 if there are transactions in a block
	}

	// update the clpa module
	for _, tx := range bInfo.InnerShardTxs {
		c.clpa.AddEdge(partition.Vertex{Addr: tx.Sender.Addr}, partition.Vertex{Addr: tx.Recipient.Addr})
	}

	for _, tx := range bInfo.Relay2Txs {
		c.clpa.AddEdge(partition.Vertex{Addr: tx.Sender.Addr}, partition.Vertex{Addr: tx.Recipient.Addr})
	}

	return nil
}

func (c *CLPARelayCommittee) ShouldStop() bool {
	return c.sl.stopCnt >= c.sl.stopThreshold
}
