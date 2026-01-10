package nodetopo

import (
	"fmt"
)

const SupervisorShardID = 0x7fffffff

type TopoGetter struct {
	leaders     map[int64]NodeInfo
	shard2nodes map[int64][]NodeInfo
}

func NewTopoGetter(l map[int64]NodeInfo, s map[int64][]NodeInfo) *TopoGetter {
	return &TopoGetter{
		leaders:     l,
		shard2nodes: s,
	}
}

func (t *TopoGetter) SetTopoGetter(infoSet map[int64]map[int64]string) error {
	t.leaders = make(map[int64]NodeInfo)
	t.shard2nodes = make(map[int64][]NodeInfo)

	for shard, shardInfo := range infoSet {
		for node := range shardInfo {
			t.shard2nodes[shard] = append(t.shard2nodes[shard], NodeInfo{ShardID: shard, NodeID: node})
			if node == 0 {
				t.leaders[shard] = NodeInfo{ShardID: shard, NodeID: node}
			}
		}
	}

	t.leaders[SupervisorShardID] = NodeInfo{ShardID: SupervisorShardID, NodeID: int64(0)}
	t.shard2nodes[SupervisorShardID] = append(
		t.shard2nodes[SupervisorShardID],
		NodeInfo{ShardID: SupervisorShardID, NodeID: int64(0)},
	)

	return nil
}

func (t *TopoGetter) GetSupervisor() (NodeInfo, error) {
	if leader, ok := t.leaders[SupervisorShardID]; ok {
		return leader, nil
	}

	return NodeInfo{}, fmt.Errorf("no supervisor found")
}

func (t *TopoGetter) GetNodesInShard(shardID int64) ([]NodeInfo, error) {
	if v, ok := t.shard2nodes[shardID]; ok {
		return v, nil
	}

	return nil, fmt.Errorf("shard %d not found", shardID)
}

func (t *TopoGetter) GetLeader(shardID int64) (NodeInfo, error) {
	if v, ok := t.leaders[shardID]; ok {
		return v, nil
	}

	return NodeInfo{}, fmt.Errorf("shard %d not found", shardID)
}

func (t *TopoGetter) ChangeLeader(shardID int64, info NodeInfo) error {
	if _, ok := t.leaders[shardID]; !ok {
		return fmt.Errorf("shard %d not found", shardID)
	}

	t.leaders[shardID] = info

	return nil
}

func (t *TopoGetter) GetAllLeaders() ([]NodeInfo, error) {
	ret := make([]NodeInfo, 0, len(t.leaders))
	for _, v := range t.leaders {
		if v.ShardID == SupervisorShardID {
			continue
		}

		ret = append(ret, v)
	}

	return ret, nil
}
