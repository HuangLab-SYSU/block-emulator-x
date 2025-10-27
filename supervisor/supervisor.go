package supervisor

import (
	"fmt"
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource/csvsource"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource/randomsource"
)

type Supervisor struct {
	logger   slog.Logger         // logger logs the information.
	r        nodetopo.NodeMapper // r give the information of other nodes.
	txSource txsource.TxSource   // txSource brings the txs into the blockchain system.
	conn     *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.

	cfg config.SupervisorCfg
}

func NewSupervisor(conn *network.P2PConn, r nodetopo.NodeMapper, cfg config.SupervisorCfg) (*Supervisor, error) {
	// choose tx-source
	var ts txsource.TxSource
	switch cfg.TxSource {
	case csvsource.CSVSourceKey:
		cs, err := csvsource.NewCSVSource(cfg.TxSourceFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create CSV source: %w", err)
		}

		ts = cs
	case randomsource.RandomSourceKey:
		ts = randomsource.NewRandomSource()
	default:
		ts = txsource.NoOperationTxSource{}
	}

	return &Supervisor{
		conn:     conn,
		r:        r,
		txSource: ts,
		cfg:      cfg,
	}, nil
}
