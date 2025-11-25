package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/committee"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/measure"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/measure/brokerstats"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/measure/relaystats"
)

const wmBufferSize = 1 << 16

type Supervisor struct {
	r    nodetopo.NodeMapper // r give the information of other nodes.
	conn *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.

	measure     measure.Measure            // measure is the stats' module.
	committee   committee.Committee        // committee controls the message sending and some consensus algorithms.
	wmBuffer    chan *rpcserver.WrappedMsg // wmBuffer is the buffer of wrapped messages.
	measureDone chan struct{}              // measureDone is to make sure that the measure subroutine will quit.

	cfg config.SupervisorCfg
}

func NewSupervisor(conn *network.P2PConn, r nodetopo.NodeMapper, cfg config.SupervisorCfg) (*Supervisor, error) {
	if meShardID := conn.GetMeNodeInfo().ShardID; meShardID != nodetopo.SupervisorShardID {
		return nil, fmt.Errorf("invalid shardID for a supervisor node, expeted=0x%x, actually=0x%x", nodetopo.SupervisorShardID, meShardID)
	}

	var (
		ms  measure.Measure
		com committee.Committee
		err error
	)

	switch cfg.ConsensusType {
	case config.StaticRelayConsensus:
		ms = relaystats.NewRelayStats()

		com, err = committee.NewStaticRelayCommittee(conn, r, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to init a StaticRelayCommittee: %w", err)
		}
	case config.StaticBrokerConsensus:
		ms = brokerstats.NewBrokerStats()

		com, err = committee.NewStaticBrokerCommittee(conn, r, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to init a StaticBrokerCommittee: %w", err)
		}
	case config.CLPARelayConsensus:
		ms = relaystats.NewRelayStats()

		com, err = committee.NewCLPARelayCommittee(conn, r, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to init a CLPARelayCommittee: %w", err)
		}
	case config.CLPABrokerConsensus:
		ms = brokerstats.NewBrokerStats()

		com, err = committee.NewCLPABrokerCommittee(conn, r, cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to init a CLPABrokerCommittee: %w", err)
		}
	default:
		return nil, fmt.Errorf("undefined consensus type: %s", cfg.ConsensusType)
	}

	// create and valid cfg output path
	if err = os.MkdirAll(cfg.ResultOutputDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create result output directory: %w", err)
	}

	return &Supervisor{
		conn: conn,
		r:    r,

		measure:     ms,
		committee:   com,
		wmBuffer:    make(chan *rpcserver.WrappedMsg, wmBufferSize),
		measureDone: make(chan struct{}),

		cfg: cfg,
	}, nil
}

func (s *Supervisor) Start() error {
	tk := time.NewTicker(time.Second)

	go s.measureSubroutine()

	defer tk.Stop()

	slog.Info("supervisor main-goroutine started")

	for range tk.C {
		if s.committee.ShouldStop() {
			break
		}

		ctx := context.Background()

		// handle messages from connections first
		msgList := s.conn.ReadMsgBuffer()

		for _, msg := range msgList {
			// handle messages in measure module, this is run in another routine (measure routine)
			s.wmBuffer <- msg
			// the messages should be handled by the committee
			if err := s.committee.HandleMsg(ctx, msg); err != nil {
				slog.ErrorContext(ctx, "failed to handle msg", "err", err)
			}
		}

		if err := s.committee.SendTxsAndConsensus(ctx); err != nil {
			slog.ErrorContext(ctx, "failed to send txs and consensus messages", "err", err)
		}
	}

	// close the wrapped message buffer, and wait all measures are handled.
	close(s.wmBuffer)
	<-s.measureDone

	// output the measure result
	if err := s.measure.OutputResult(s.cfg.ResultOutputDir); err != nil {
		slog.Error("failed to output result", "err", err)
	} else {
		slog.Info("successfully output the result", "dir", s.cfg.ResultOutputDir)
	}

	// Send 'stop consensus message' to the consensus nodes.
	wMsg, err := message.WrapMsg(&message.StopConsensusMsg{})
	if err != nil {
		return fmt.Errorf("failed to wrap stop consensus message: %w", err)
	}

	destNodes := make([]nodetopo.NodeInfo, 0)

	for i := range s.cfg.ShardNum {
		ls, err := s.r.GetNodesInShard(i)
		if err != nil {
			return fmt.Errorf("get all leaders failed when trying to send stop: %w", err)
		}

		destNodes = append(destNodes, ls...)
	}

	s.conn.GroupBroadcastMessage(context.Background(), destNodes, wMsg)

	return nil
}

func (s *Supervisor) measureSubroutine() {
	slog.Info("supervisor measure subroutine started")

	for wm := range s.wmBuffer {
		if err := s.measure.UpdateMeasureRecord(wm); err != nil {
			slog.Error("failed to update measure record", "err", err)
		}
	}

	s.measureDone <- struct{}{}
}
