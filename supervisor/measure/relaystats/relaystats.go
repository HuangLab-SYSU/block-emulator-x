package relaystats

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"slices"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/csvwrite"
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

const (
	detailTxInfoPath = "relay_stats_detail_tx_info.csv"
	briefTxInfoPath  = "relay_stats_brief_info.csv"
)

var detailTxInfoMeasures = []string{
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

type RelayStats struct {
	// TCL, i.e., Transaction Commit Latency
	relay1TCLSum, relay2TCLSum map[int]time.Duration // the commit latency sum of relay1/relay2 transactions in each epoch
	innerShardTCLSum           map[int]time.Duration // the commit latency sum of inner-shard transactions in each epoch

	// The number of transactions for each epoch
	innerShardTxNum          map[int]int // the number of inner-shard transactions in each epoch
	relay1TxNum, relay2TxNum map[int]int // the number of relay1/relay2 transactions in each epoch

	epochStartTime, epochEndTime map[int]time.Time // the start/end time for each epoch

	txLifecycles map[string]*txLifeCycle // the lifecycle of all transactions

	outputDir string
	cs        *csvwrite.CSVSeqWriter
}

func NewRelayStats(outputDir string) (*RelayStats, error) {
	fp := filepath.Join(outputDir, detailTxInfoPath)

	cs, err := csvwrite.NewCSVSeqWriter(fp, detailTxInfoMeasures)
	if err != nil {
		return nil, fmt.Errorf("error creating CSV sequence writer: %w", err)
	}

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
		outputDir:        outputDir,
		cs:               cs,
	}, nil
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
		th, err := tx.Hash()
		if err != nil {
			slog.Error("invalid hash", "CalcHash err", err)
			continue
		}

		r.txLifecycles[string(th)] = &txLifeCycle{
			originalTxCreateTime:         tx.CreateTime,
			innerShardTxBlockProposeTime: bInfo.BlockProposeTime,
			originalTxCommitTime:         bInfo.BlockCommitTime,
			isCrossShardTx:               false,
		}

		r.innerShardTCLSum[epochID] += bInfo.BlockCommitTime.Sub(tx.CreateTime)
		if err = r.writeTxInfo(th, r.txLifecycles[string(th)]); err != nil {
			slog.Error("writeTxInfo (relay tx) failed", "err", err)
		}

		delete(r.txLifecycles, string(th))
	}

	for _, tx := range bInfo.Relay1Txs {
		// set relay 1 to the pair map
		strTxHash := string(tx.ROriginalHash)

		_, r2Exist := r.txLifecycles[strTxHash]
		if !r2Exist {
			r.txLifecycles[strTxHash] = &txLifeCycle{originalTxCreateTime: tx.CreateTime, isCrossShardTx: true}
		}

		r.txLifecycles[strTxHash].relay1BlockProposeTime = bInfo.BlockProposeTime
		r.txLifecycles[strTxHash].relay1CommitTime = bInfo.BlockCommitTime

		if r2Exist { // If the relay2 tx exists, the txLifecycle of this tx is filled because this relay1 tx.
			r.updateRelayTxTCLByTxLifecycle(r.txLifecycles[strTxHash], epochID)

			if err := r.writeTxInfo(tx.ROriginalHash, r.txLifecycles[strTxHash]); err != nil {
				slog.Error("writeTxInfo (relay tx) failed", "err", err)
			}

			delete(r.txLifecycles, strTxHash)
		}
	}

	for _, tx := range bInfo.Relay2Txs {
		strTxHash := string(tx.ROriginalHash)

		_, r1Exist := r.txLifecycles[strTxHash]
		if !r1Exist {
			r.txLifecycles[strTxHash] = &txLifeCycle{originalTxCreateTime: tx.CreateTime, isCrossShardTx: true}
		}

		r.txLifecycles[strTxHash].relay2BlockProposeTime = bInfo.BlockProposeTime
		r.txLifecycles[strTxHash].relay2CommitTime = bInfo.BlockCommitTime
		r.txLifecycles[strTxHash].originalTxCommitTime = bInfo.BlockCommitTime

		if r1Exist { // If the relay1 tx exists, the txLifecycle of this tx is filled because this relay2 tx.
			r.updateRelayTxTCLByTxLifecycle(r.txLifecycles[strTxHash], epochID)

			if err := r.writeTxInfo(tx.ROriginalHash, r.txLifecycles[strTxHash]); err != nil {
				slog.Error("writeTxInfo (relay tx) failed", "err", err)
			}

			delete(r.txLifecycles, strTxHash)
		}
	}

	return nil
}

func (r *RelayStats) OutputResultAndClose() error {
	briefInfoFp := filepath.Join(r.outputDir, briefTxInfoPath)
	if err := r.outputBriefEpochInfo(briefInfoFp); err != nil {
		return fmt.Errorf("failed to output BriefEpochInfo: %w", err)
	}

	slog.Info("relay stats has output all results")

	return r.cs.Close()
}

func (r *RelayStats) updateRelayTxTCLByTxLifecycle(tl *txLifeCycle, epochID int) {
	r1TCL, r2TCL := tl.relay1CommitTime.Sub(tl.originalTxCreateTime), tl.relay2CommitTime.Sub(tl.relay1CommitTime)
	r.relay1TCLSum[epochID] += r1TCL
	r.relay2TCLSum[epochID] += r2TCL
}

func (r *RelayStats) writeTxInfo(txHash []byte, tl *txLifeCycle) error {
	csvLine := []string{
		hex.EncodeToString(txHash),
		utils.ConvertTime2Str(tl.originalTxCreateTime),
		utils.ConvertTime2Str(tl.originalTxCommitTime),
		fmt.Sprintf("%t", tl.isCrossShardTx),
		utils.ConvertTime2Str(tl.innerShardTxBlockProposeTime),
		utils.ConvertTime2Str(tl.relay1BlockProposeTime),
		utils.ConvertTime2Str(tl.relay1CommitTime),
		utils.ConvertTime2Str(tl.relay2BlockProposeTime),
		utils.ConvertTime2Str(tl.relay2CommitTime),
	}
	if err := r.cs.WriteLine2CSV(csvLine); err != nil {
		return fmt.Errorf("WriteLine2CSV failed: %w", err)
	}

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

	return csvwrite.WriteAllToCSV(fp, measureName, measureVals)
}
