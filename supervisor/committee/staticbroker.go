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
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource"
)

type StaticBrokerCommittee struct {
	r    nodetopo.NodeMapper // r give the information of other nodes.
	conn *network.P2PConn    // conn is the p2p-connections among consensus nodes, i.e., network layer.

	txSource    txsource.TxSource // txSource brings the txs into the blockchain system.
	sl          stopLogic         // sl is the logic of stop.
	unsentTxNum int64

	cfg config.SupervisorCfg
}

func NewStaticBrokerCommittee(conn *network.P2PConn, r nodetopo.NodeMapper, cfg config.SupervisorCfg) (*StaticBrokerCommittee, error) {
	return &StaticBrokerCommittee{}, nil
}

func (s StaticBrokerCommittee) SendTxsAndConsensus(ctx context.Context) error {
	// TODO implement me
	panic("implement me")
}

func (s StaticBrokerCommittee) HandleMsg(ctx context.Context, msg *rpcserver.WrappedMsg) error {
	if msg.GetMsgType() != message.BrokerBlockInfoMessageType {
		return fmt.Errorf("unexpected msg type: %s", msg.GetMsgType())
	}

	var bInfo message.BrokerBlockInfoMsg
	if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&bInfo); err != nil {
		return fmt.Errorf("decode relayBlockInfoMsg: %w", err)
	}

	// update the stop module
	if len(bInfo.InnerShardTxs)+len(bInfo.Broker1Txs)+len(bInfo.Broker2Txs) == 0 {
		s.sl.stopCnt++
	} else {
		s.sl.stopCnt = 0 // reset 0 if there are transactions in a block
	}

	// operate as a broker
	panic("implement me")
}

func (s StaticBrokerCommittee) ShouldStop() bool {
	return s.sl.stopCnt >= s.sl.stopThreshold
}
