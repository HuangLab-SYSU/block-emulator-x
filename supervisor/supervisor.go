package supervisor

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/committee"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/measure"
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

	for range tk.C {
		if s.committee.ShouldStop() {
			break
		}

		ctx := context.Background()

		// handle messages from connections first
		msgList := s.conn.ReadMsgBuffer()

		for _, msg := range msgList {
			// handle messages in measure module, this is run in another routine
			s.wmBuffer <- msg
			// the messages should be handled by the committee
			if err := s.committee.HandleMsg(ctx, msg); err != nil {
				slog.ErrorContext(ctx, "failed to handle msg", "err", err)
			}
		}
	}

	close(s.wmBuffer)
	<-s.measureDone

	// output the measure result
	if err := s.measure.OutputResult(s.cfg.ResultOutputDir); err != nil {
		slog.Error("failed to output result", "err", err)
	} else {
		slog.Info("successfully output the result", "dir", s.cfg.ResultOutputDir)
	}

	return nil
}

func (s *Supervisor) measureSubroutine() {
	for wm := range s.wmBuffer {
		if err := s.measure.UpdateMeasureRecord(wm); err != nil {
			slog.Error("failed to update measure record", "err", err)
		}
	}

	s.measureDone <- struct{}{}
}
