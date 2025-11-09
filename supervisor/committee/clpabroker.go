package committee

import (
	"context"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/nodetopo"
)

type CLPABrokerCommittee struct{}

func NewCLPABrokerCommittee(conn *network.P2PConn, r nodetopo.NodeMapper, cfg config.SupervisorCfg) (*CLPABrokerCommittee, error) {
	return &CLPABrokerCommittee{}, nil
}

func (c *CLPABrokerCommittee) SendTxsAndConsensus(ctx context.Context) error {
	// TODO implement me
	panic("implement me")
}

func (c *CLPABrokerCommittee) HandleMsg(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	// TODO implement me
	panic("implement me")
}

func (c *CLPABrokerCommittee) ShouldStop() bool {
	// TODO implement me
	panic("implement me")
}
