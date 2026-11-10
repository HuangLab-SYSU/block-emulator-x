package committee

import (
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/partition"
)

// partitionRunner maintains the states of CLPAState and records the consensus states of the partition algorithm.
type partitionRunner struct {
	state           *partition.CLPAState // state is the module of CLPA, aka., an account-reallocation algorithm.
	lastRunTime     time.Time
	epochSynced     bool    // labels that the epochs of the supervisor and all shards are synced (i.e., the clpa round is done).
	supervisorEpoch int64   // supervisorEpoch is the epoch ID of this supervisor.
	shardEpoch      []int64 // shardEpoch tells the epoch ID for each shard.
}

// clpaVisitOrder returns the visit order of the repartition about to run. supervisorEpoch has not
// been incremented yet, so +1 is the epoch this partition actually takes effect in.
func (p *partitionRunner) clpaVisitOrder(seed int64) partition.VisitOrder {
	return partition.SeededOrder(seed + p.supervisorEpoch + 1)
}

// CheckEpochSyncAndMark tells whether all shards are in the correct epoch
func (p *partitionRunner) CheckEpochSyncAndMark() bool {
	// if the epoch is synced, return
	if p.epochSynced {
		return true
	}

	// if the epoch is not synced, retry to compute it.
	for _, epochID := range p.shardEpoch {
		if epochID != p.supervisorEpoch {
			return false
		}
	}

	// the epoch is synced, record the last run time
	p.epochSynced = true
	p.lastRunTime = time.Now()

	return true
}
