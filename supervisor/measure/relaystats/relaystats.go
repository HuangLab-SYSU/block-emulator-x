package relaystats

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
	originalTxCreateTime, originalTxCommitTime     time.Time
	innerShardTxBlockProposeTime                   time.Time
	relay1BlockProposeTime, relay2BlockProposeTime time.Time
	relay1CommitTime, relay2CommitTime             time.Time
	isCrossShardTx                                 bool
}

type RelayStats struct {
	// TCL, i.e., Transaction Commit Latency
	relay1TCLSum, relay2TCLSum map[int]time.Duration // the commit latency sum of relay1/relay2 transactions in each epoch
	innerShardTCLSum           map[int]time.Duration // the commit latency sum of inner-shard transactions in each epoch

	// The number of transactions for each epoch
	innerShardTxNum          map[int]int // the number of inner-shard transactions in each epoch
	relay1TxNum, relay2TxNum map[int]int // the number of relay1/relay2 transactions in each epoch

	epochStartTime, epochEndTime map[int]time.Time // the start/end time for each epoch

	txLifecycles map[string]*txLifeCycle // the lifecycle of all transactions
}

func NewRelayStats() *RelayStats {
	return &RelayStats{
		relay1TCLSum:     make(map[int]time.Duration),
		relay2TCLSum:     make(map[int]time.Duration),
		innerShardTCLSum: make(map[int]time.Duration),
		innerShardTxNum:  make(map[int]int),
		relay1TxNum:      make(map[int]int),
		relay2TxNum:      make(map[int]int),
		epochStartTime:   make(map[int]time.Time),
		epochEndTime:     make(map[int]time.Time),
		txLifecycles:     make(map[string]*txLifeCycle),
	}
}

func (r *RelayStats) UpdateMeasureRecord(msg *rpcserver.WrappedMsg) error {
	// ignore
	if msg.MsgType != message.RelayBlockInfoMessageType {
		slog.Error("Unsupported message type: ", "type", msg.MsgType)
		return nil
	}

	var bInfo message.RelayBlockInfoMsg
	if err := gob.NewDecoder(bytes.NewReader(msg.GetPayload())).Decode(&bInfo); err != nil {
		return fmt.Errorf("decode relayBlockInfoMsg: %w", err)
	}

	slog.Info("relay stats: receives the block info message", "from shardID", bInfo.ShardID, "epoch", bInfo.Epoch)

	epochID := int(bInfo.Epoch)
	// update the start/end time of this epoch
	if start, ok := r.epochStartTime[epochID]; !ok || bInfo.BlockProposeTime.Before(start) {
		r.epochStartTime[epochID] = bInfo.BlockProposeTime
	}

	if end, ok := r.epochEndTime[epochID]; !ok || bInfo.BlockCommitTime.After(end) {
		r.epochEndTime[epochID] = bInfo.BlockCommitTime
	}

	// update the number of all maps
	r.relay1TxNum[epochID] += len(bInfo.Relay1Txs)
	r.relay2TxNum[epochID] += len(bInfo.Relay2Txs)
	r.innerShardTxNum[epochID] += len(bInfo.InnerShardTxs)

	// update the sum of latency
	for _, tx := range bInfo.InnerShardTxs {
		th, err := utils.CalcHash(&tx)
		if err != nil {
			slog.Error("invalid hash", "CalcHash err", err)
			continue
		}

		r.txLifecycles[string(th)] = &txLifeCycle{
			originalTxCreateTime:         tx.CreateTime,
			innerShardTxBlockProposeTime: bInfo.BlockProposeTime,
			originalTxCommitTime:         bInfo.BlockCommitTime,
		}

		r.innerShardTCLSum[epochID] += bInfo.BlockCommitTime.Sub(tx.CreateTime)
	}

	for _, tx := range bInfo.Relay1Txs {
		// set relay 1 to the pair map
		strTxHash := string(tx.ROriginalHash)
		if val := r.txLifecycles[strTxHash]; val == nil {
			r.txLifecycles[strTxHash] = &txLifeCycle{
				originalTxCreateTime: tx.CreateTime,
				isCrossShardTx:       true,
			}
		}

		r.txLifecycles[strTxHash].relay1BlockProposeTime = bInfo.BlockProposeTime
		r.txLifecycles[strTxHash].relay1CommitTime = bInfo.BlockCommitTime

		r.relay1TCLSum[epochID] += bInfo.BlockCommitTime.Sub(tx.CreateTime)
	}

	for _, tx := range bInfo.Relay2Txs {
		strTxHash := string(tx.ROriginalHash)
		if val := r.txLifecycles[strTxHash]; val == nil {
			r.txLifecycles[strTxHash] = &txLifeCycle{
				originalTxCreateTime: tx.CreateTime,
				isCrossShardTx:       true,
			}
		}

		r.txLifecycles[strTxHash].relay2BlockProposeTime = bInfo.BlockProposeTime
		r.txLifecycles[strTxHash].relay2CommitTime = bInfo.BlockCommitTime
		r.txLifecycles[strTxHash].originalTxCommitTime = bInfo.BlockCommitTime

		r.relay2TCLSum[epochID] += bInfo.BlockCommitTime.Sub(tx.CreateTime)
	}

	return nil
}

func (r *RelayStats) OutputResult(fp string) error {
	briefInfoFp := path.Join(fp, "relay_stats_brief_info.csv")
	if err := r.outputBriefEpochInfo(briefInfoFp); err != nil {
		return fmt.Errorf("failed to output BriefEpochInfo: %w", err)
	}

	detailTxInfoFp := path.Join(fp, "relay_stats_detail_tx_info.csv")
	if err := r.outputDetailTxInfo(detailTxInfoFp); err != nil {
		return fmt.Errorf("failed to outputDetailTxInfo: %w", err)
	}

	slog.Info("relay stats has output all results")

	return nil
}

func (r *RelayStats) outputBriefEpochInfo(fp string) error {
	slog.Info("output relay stats", "metric", "brief epoch info", "file", fp)

	measureName := []string{
		"EpochID",
		"Total tx # in this epoch",
		"Inner-shard tx # in this epoch",
		"Relay1 tx # in this epoch",
		"Relay2 tx # in this epoch",
		"Epoch start time",
		"Epoch end time",
		"Avg. TPS of this epoch (txs per second)",
		"CTX ratio of this epoch",
		"Avg. TCL of this epoch (second)",
		"Avg. inner-shard TCL of this epoch (second)",
		"Avg. relay1 TCL of this epoch (second)",
		"Avg. relay2 TCL of this epoch (second)",
	}

	epochIDs := slices.Sorted(maps.Keys(r.epochStartTime))
	measureVals := make([][]string, 0, len(epochIDs))

	for _, epochID := range epochIDs {
		epochDuration := r.epochEndTime[epochID].Sub(r.epochStartTime[epochID]).Seconds()
		ctxCnt := float64(r.relay1TxNum[epochID]+r.relay2TxNum[epochID]) / 2.0
		totalTxCnt := float64(r.innerShardTxNum[epochID]) + ctxCnt

		relay1TCL, relay2TCL := float64(r.relay1TCLSum[epochID]), float64(r.relay2TCLSum[epochID])
		innerShardTCL := float64(r.innerShardTCLSum[epochID])
		totalTCL := relay1TCL + relay2TCL + innerShardTCL

		csvLine := []string{
			fmt.Sprintf("%d", epochID),
			fmt.Sprintf("%.2f", totalTxCnt),
			fmt.Sprintf("%d", r.innerShardTxNum[epochID]),
			fmt.Sprintf("%d", r.relay1TxNum[epochID]),
			fmt.Sprintf("%d", r.relay2TxNum[epochID]),
			r.epochStartTime[epochID].Format(time.RFC3339),
			r.epochEndTime[epochID].Format(time.RFC3339),
			fmt.Sprintf("%.2f", totalTxCnt/epochDuration),
			fmt.Sprintf("%.2f", ctxCnt/totalTxCnt),
			fmt.Sprintf("%.2f", totalTCL/totalTxCnt),
			fmt.Sprintf("%.2f", innerShardTCL/float64(r.innerShardTxNum[epochID])),
			fmt.Sprintf("%.2f", relay1TCL/float64(r.relay1TxNum[epochID])),
			fmt.Sprintf("%.2f", relay2TCL/float64(r.relay2TxNum[epochID])),
		}
		measureVals = append(measureVals, csvLine)
	}

	return utils.WriteAllToCSV(fp, measureName, measureVals)
}

func (r *RelayStats) outputDetailTxInfo(fp string) error {
	slog.Info("output relay stats", "metric", "detail tx info", "file", fp)

	measureName := []string{
		"OriginalHash",
		"Tx create time",
		"Tx finally commit time",
		"Is cross-shard tx or not",
		"Inner shard tx block propose time",
		"Relay1 block propose time",
		"Relay1 tx commit time",
		"Relay2 block propose time",
		"Relay2 tx commit time",
	}

	measureVals := make([][]string, 0, len(r.txLifecycles))
	for hash, txDetail := range r.txLifecycles {
		csvLine := []string{
			hex.EncodeToString([]byte(hash)),
			utils.ConvertTime2Str(txDetail.originalTxCreateTime),
			utils.ConvertTime2Str(txDetail.originalTxCommitTime),
			fmt.Sprintf("%t", txDetail.isCrossShardTx),
			utils.ConvertTime2Str(txDetail.innerShardTxBlockProposeTime),
			utils.ConvertTime2Str(txDetail.relay1BlockProposeTime),
			utils.ConvertTime2Str(txDetail.relay1CommitTime),
			utils.ConvertTime2Str(txDetail.relay2BlockProposeTime),
			utils.ConvertTime2Str(txDetail.relay2CommitTime),
		}
		measureVals = append(measureVals, csvLine)
	}

	return utils.WriteAllToCSV(fp, measureName, measureVals)
}
