package committee

import (
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/partition"
)

type clpaComponent struct {
	state           *partition.CLPAState // state is the module of CLPA, aka., an account-reallocation algorithm.
	lastRunTime     time.Time
	epochSynced     bool    // labels that the epochs of the supervisor and all shards are synced (i.e., the clpa round is done).
	supervisorEpoch int64   // supervisorEpoch is the epoch ID of this supervisor.
	shardEpoch      []int64 // shardEpoch tells the epoch ID for each shard.
}

// checkEpochSyncAndMark tells whether all shards are in the correct epoch
func (clpa *clpaComponent) checkEpochSyncAndMark() bool {
	// if the epoch is synced, return
	if clpa.epochSynced {
		return true
	}

	// if the epoch is not synced, retry to compute it.
	for _, epochID := range clpa.shardEpoch {
		if epochID != clpa.supervisorEpoch {
			return false
		}
	}

	// the epoch is synced, record the last run time
	clpa.epochSynced = true
	clpa.lastRunTime = time.Now()

	return true
}
