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
