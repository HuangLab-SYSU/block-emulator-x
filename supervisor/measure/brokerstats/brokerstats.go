package brokerstats

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log/slog"
	"maps"
	"path"
	"slices"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/message"
	"github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
)

type txLifeCycle struct {
	originalTxCreateTime, originalTxCommitTime       time.Time
	innerShardTxBlockProposeTime                     time.Time
	broker1TxCreateTime, broker2TxCreateTime         time.Time
	broker1BlockProposeTime, broker2BlockProposeTime time.Time
	broker1CommitTime, broker2CommitTime             time.Time
	isBrokerTx                                       bool
}

type BrokerStats struct {
	// TCL, i.e., Transaction Commit Latency
	broker1TCLSum, broker2TCLSum map[int]time.Duration // the commit latency sum of broker1/broker2 transactions in each epoch
	innerShardTCLSum             map[int]time.Duration // the commit latency sum of inner-shard transactions in each epoch

	// The number of transactions for each epoch
	innerShardTxNum            map[int]int // the number of inner-shard transactions in each epoch
	broker1TxNum, broker2TxNum map[int]int // the number of broker1/broker2 transactions in each epoch

	epochStartTime, epochEndTime map[int]time.Time // the start/end time for each epoch

	txLifecycles map[string]*txLifeCycle // the lifecycle of all transactions
}

func NewBrokerStats() *BrokerStats {
	return &BrokerStats{
		broker1TCLSum:    make(map[int]time.Duration),
		broker2TCLSum:    make(map[int]time.Duration),
		innerShardTCLSum: make(map[int]time.Duration),
		innerShardTxNum:  make(map[int]int),
		broker1TxNum:     make(map[int]int),
		broker2TxNum:     make(map[int]int),
		epochStartTime:   make(map[int]time.Time),
		epochEndTime:     make(map[int]time.Time),
		txLifecycles:     make(map[string]*txLifeCycle),
	}
}

func (b *BrokerStats) UpdateMeasureRecord(msg *rpcserver.WrappedMsg) error {
	// ignore
	if msg.MsgType != message.BrokerBlockInfoMessageType {
		return fmt.Errorf("UpdateMeasureRecord failed, wrong message type: %s", msg.MsgType)
	}

	var bInfo message.BrokerBlockInfoMsg
	if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&bInfo); err != nil {
		return fmt.Errorf("decode brokerBlockInfoMsg: %w", err)
	}

	epochID := int(bInfo.Epoch)

	// update the start/end time of this epoch
	if start, ok := b.epochStartTime[epochID]; !ok || bInfo.BlockProposeTime.Before(start) {
		b.epochStartTime[epochID] = bInfo.BlockProposeTime
	}

	if end, ok := b.epochEndTime[epochID]; !ok || bInfo.BlockCommitTime.After(end) {
		b.epochEndTime[epochID] = bInfo.BlockCommitTime
	}

	// update the number of all maps
	b.broker1TxNum[epochID] += len(bInfo.Broker1Txs)
	b.broker2TxNum[epochID] += len(bInfo.Broker2Txs)
	b.innerShardTxNum[epochID] += len(bInfo.InnerShardTxs)

	// update the sum of latency
	for _, tx := range bInfo.InnerShardTxs {
		th, err := utils.CalcHash(&tx)
		if err != nil {
			slog.Error("invalid hash", "CalcHash err", err)
			continue
		}

		b.txLifecycles[string(th)] = &txLifeCycle{
			originalTxCreateTime:         tx.CreateTime,
			innerShardTxBlockProposeTime: bInfo.BlockProposeTime,
			originalTxCommitTime:         bInfo.BlockCommitTime,
		}

		b.innerShardTCLSum[epochID] += bInfo.BlockCommitTime.Sub(tx.CreateTime)
	}

	for _, tx := range bInfo.Broker1Txs {
		// set broker 1 to the pair map
		strTxHash := string(tx.BOriginalHash)
		if val := b.txLifecycles[strTxHash]; val == nil {
			b.txLifecycles[strTxHash] = &txLifeCycle{
				originalTxCreateTime: tx.OriginalTxCreateTime,
				isBrokerTx:           true,
			}
		}

		b.txLifecycles[strTxHash].broker1TxCreateTime = tx.CreateTime
		b.txLifecycles[strTxHash].broker1BlockProposeTime = bInfo.BlockProposeTime
		b.txLifecycles[strTxHash].broker1CommitTime = bInfo.BlockCommitTime

		b.broker1TCLSum[epochID] += bInfo.BlockCommitTime.Sub(tx.CreateTime)
	}

	for _, tx := range bInfo.Broker2Txs {
		strTxHash := string(tx.BOriginalHash)
		if val := b.txLifecycles[strTxHash]; val == nil {
			b.txLifecycles[strTxHash] = &txLifeCycle{
				originalTxCreateTime: tx.OriginalTxCreateTime,
				isBrokerTx:           true,
			}
		}

		b.txLifecycles[strTxHash].broker2TxCreateTime = tx.CreateTime
		b.txLifecycles[strTxHash].broker2BlockProposeTime = bInfo.BlockProposeTime
		b.txLifecycles[strTxHash].broker2CommitTime = bInfo.BlockCommitTime
		b.txLifecycles[strTxHash].originalTxCommitTime = bInfo.BlockCommitTime

		b.broker2TCLSum[epochID] += bInfo.BlockCommitTime.Sub(tx.CreateTime)
	}

	return nil
}

// OutputResult outputs the metrics of all messages
func (b *BrokerStats) OutputResult(fp string) error {
	briefInfoFp := path.Join(fp, "broker_stats_brief_info.csv")
	if err := b.outputBriefEpochInfo(briefInfoFp); err != nil {
		return fmt.Errorf("failed to output BriefEpochInfo: %w", err)
	}

	detailTxInfoFp := path.Join(fp, "broker_stats_detail_tx_info.csv")
	if err := b.outputDetailTxInfo(detailTxInfoFp); err != nil {
		return fmt.Errorf("failed to outputDetailTxInfo: %w", err)
	}

	return nil
}

func (b *BrokerStats) outputBriefEpochInfo(fp string) error {
	slog.Info("output broker stats", "metric", "brief epoch info", "file", fp)

	measureName := []string{
		"EpochID",
		"Total tx # in this epoch",
		"Inner-shard tx # in this epoch",
		"Broker1 tx # in this epoch",
		"Broker2 tx # in this epoch",
		"Epoch start time",
		"Epoch end time",
		"Avg. TPS of this epoch (txs per second)",
		"CTX ratio of this epoch",
		"Avg. TCL of this epoch (second)",
		"Avg. inner-shard TCL of this epoch (second)",
		"Avg. broker1 TCL of this epoch (second)",
		"Avg. broker2 TCL of this epoch (second)",
	}

	epochIDs := slices.Sorted(maps.Keys(b.epochStartTime))
	measureVals := make([][]string, 0, len(epochIDs))

	for _, epochID := range epochIDs {
		epochDuration := b.epochEndTime[epochID].Sub(b.epochStartTime[epochID]).Seconds()
		ctxCnt := float64(b.broker1TxNum[epochID]+b.broker2TxNum[epochID]) / 2.0
		totalTxCnt := float64(b.innerShardTxNum[epochID]) + ctxCnt

		broker1TCL, broker2TCL := float64(b.broker1TCLSum[epochID]), float64(b.broker2TCLSum[epochID])
		innerShardTCL := float64(b.innerShardTCLSum[epochID])
		totalTCL := broker1TCL + broker2TCL + innerShardTCL

		csvLine := []string{
			fmt.Sprintf("%d", epochID),
			fmt.Sprintf("%.2f", totalTxCnt),
			fmt.Sprintf("%d", b.innerShardTxNum[epochID]),
			fmt.Sprintf("%d", b.broker1TxNum[epochID]),
			fmt.Sprintf("%d", b.broker2TxNum[epochID]),
			b.epochStartTime[epochID].Format(time.RFC3339),
			b.epochEndTime[epochID].Format(time.RFC3339),
			fmt.Sprintf("%.2f", totalTxCnt/epochDuration),
			fmt.Sprintf("%.2f", ctxCnt/totalTxCnt),
			fmt.Sprintf("%.2f", totalTCL/totalTxCnt),
			fmt.Sprintf("%.2f", broker1TCL/float64(b.broker1TxNum[epochID])),
			fmt.Sprintf("%.2f", broker2TCL/float64(b.broker2TxNum[epochID])),
			fmt.Sprintf("%.2f", innerShardTCL/float64(b.innerShardTxNum[epochID])),
		}
		measureVals = append(measureVals, csvLine)
	}

	return utils.WriteAllToCSV(fp, measureName, measureVals)
}

func (b *BrokerStats) outputDetailTxInfo(fp string) error {
	slog.Info("output broker stats", "metric", "detail tx info", "file", fp)

	measureName := []string{
		"OriginalHash",
		"Tx create time",
		"Tx finally commit time",
		"Is broker tx or not",
		"Inner shard tx block propose time",
		"Broker1 tx create time",
		"Broker1 block propose time",
		"Broker1 tx commit time",
		"Broker2 tx create time",
		"Broker2 block propose time",
		"Broker2 tx commit time",
	}

	measureVals := make([][]string, 0, len(b.txLifecycles))
	for hash, txDetail := range b.txLifecycles {
		csvLine := []string{
			hex.EncodeToString([]byte(hash)),
			utils.ConvertTime2Str(txDetail.originalTxCreateTime),
			utils.ConvertTime2Str(txDetail.originalTxCommitTime),
			fmt.Sprintf("%t", txDetail.isBrokerTx),
			utils.ConvertTime2Str(txDetail.innerShardTxBlockProposeTime),
			utils.ConvertTime2Str(txDetail.broker1TxCreateTime),
			utils.ConvertTime2Str(txDetail.broker1BlockProposeTime),
			utils.ConvertTime2Str(txDetail.broker1CommitTime),
			utils.ConvertTime2Str(txDetail.broker2TxCreateTime),
			utils.ConvertTime2Str(txDetail.broker2BlockProposeTime),
			utils.ConvertTime2Str(txDetail.broker2CommitTime),
		}
		measureVals = append(measureVals, csvLine)
	}

	return utils.WriteAllToCSV(fp, measureName, measureVals)
}
