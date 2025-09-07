package shard

import (
	"context"
	"encoding/binary"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/core/account"
	"github.com/HuangLab-SYSU/block-emulator/utils"
)

type FixNumShardResolver struct {
	shardNumber int64
	shardList   []*Shard
	nodeId2Node map[TypeNodeId]*Node

	shardNodeBiMap    *utils.BiMultiMap[TypeShardId, TypeNodeId]
	shardAccountBiMap *utils.BiMultiMap[TypeShardId, account.Address]
}

func NewFixNumShardResolver(shardNum int64) (*FixNumShardResolver, error) {
	f := &FixNumShardResolver{
		shardNumber: shardNum,
		shardList:   make([]*Shard, shardNum),
		nodeId2Node: make(map[TypeNodeId]*Node),

		shardNodeBiMap:    utils.NewBiMultiMap[TypeShardId, TypeNodeId](),
		shardAccountBiMap: utils.NewBiMultiMap[TypeShardId, account.Address](),
	}
	return f, nil
}

func (f *FixNumShardResolver) GetShardByShardId(_ context.Context, shardId TypeShardId) (*Shard, error) {
	if !f.validateShardId(shardId) {
		return nil, fmt.Errorf("validate shard id failed, shardId=%d", shardId)
	}
	if f.shardList[shardId] == nil {
		return nil, fmt.Errorf("shard id not found, shardId=%d", shardId)
	}
	return f.shardList[shardId], nil
}

func (f *FixNumShardResolver) GetNodeIdsInShard(_ context.Context, shardId TypeShardId) ([]TypeNodeId, error) {
	if !f.validateShardId(shardId) {
		return nil, fmt.Errorf("validate shard id failed, shardId=%d", shardId)
	}

	nodeIds := f.shardNodeBiMap.GetByKey(shardId)
	return nodeIds, nil
}

func (f *FixNumShardResolver) AddShard(_ context.Context, shard Shard) error {
	shardId := shard.Id
	if !f.validateShardId(shardId) {
		return fmt.Errorf("validate shard id failed, shardId=%d", shardId)
	}
	if f.shardList[shardId] != nil {
		return fmt.Errorf("shard exists, shardId=%d", shardId)
	}

	f.shardList[shardId] = &shard
	return nil
}

func (f *FixNumShardResolver) CloseShard(_ context.Context, shardId TypeShardId) error {
	if !f.validateShardId(shardId) {
		return fmt.Errorf("validate shard id failed, shardId=%d", shardId)
	}

	f.shardList[shardId] = nil
	f.shardNodeBiMap.RemoveByKey(shardId)
	return nil
}

func (f *FixNumShardResolver) GetNodeByNodeId(_ context.Context, nodeId TypeNodeId) (*Node, error) {
	if _, ok := f.nodeId2Node[nodeId]; !ok {
		return nil, fmt.Errorf("node not exists, nodeId=%d", nodeId)
	}
	return f.nodeId2Node[nodeId], nil
}

func (f *FixNumShardResolver) GetLocShardIdsByNodeId(_ context.Context, nodeId TypeNodeId) ([]TypeShardId, error) {
	shardIds := f.shardNodeBiMap.GetByValue(nodeId)
	return shardIds, nil
}

func (f *FixNumShardResolver) AddNodeToShard(_ context.Context, node Node, destShardId TypeShardId) error {
	if !f.validateShardId(destShardId) {
		return fmt.Errorf("validate shard id failed, shardId=%d", destShardId)
	}
	if _, exist := f.nodeId2Node[node.Id]; exist {
		return fmt.Errorf("node exists, nodeId=%d", node.Id)
	}
	f.nodeId2Node[node.Id] = &node
	f.shardNodeBiMap.Add(destShardId, node.Id)
	return nil
}

func (f *FixNumShardResolver) DeleteNode(_ context.Context, nodeId TypeNodeId) error {
	f.nodeId2Node[nodeId] = nil
	f.shardNodeBiMap.RemoveByValue(nodeId)
	return nil
}

func (f *FixNumShardResolver) GetLocShardsIdByAccountAddr(_ context.Context, addr account.Address) ([]TypeShardId, error) {
	shardIds := f.shardAccountBiMap.GetByValue(addr)
	if len(shardIds) == 0 {
		return []TypeShardId{f.getDefaultShard(addr)}, nil
	}
	return shardIds, nil
}

func (f *FixNumShardResolver) AddAccountToShard(_ context.Context, addr account.Address, destShardId TypeShardId) error {
	if !f.validateShardId(destShardId) {
		return fmt.Errorf("validate shard id failed, shardId=%d", destShardId)
	}
	f.shardAccountBiMap.Add(destShardId, addr)
	return nil
}

func (f *FixNumShardResolver) DeleteAccountInShard(_ context.Context, addr account.Address, shardId TypeShardId) error {
	if !f.validateShardId(shardId) {
		return fmt.Errorf("validate shard id failed, shardId=%d", shardId)
	}
	f.shardAccountBiMap.RemoveByValue(addr)
	return nil
}

func (f *FixNumShardResolver) validateShardId(shardId TypeShardId) bool {
	return shardId >= 0 && int64(shardId) < f.shardNumber
}

func (f *FixNumShardResolver) getDefaultShard(addr account.Address) TypeShardId {
	var buf [8]byte
	copy(buf[4:], addr[16:]) // 4 bytes
	u64 := binary.BigEndian.Uint64(buf[:])
	return TypeShardId(u64 % uint64(f.shardNumber))
}
