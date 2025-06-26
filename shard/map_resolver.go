package shard

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/HuangLab-SYSU/block-emulator/core/account"
	"github.com/HuangLab-SYSU/block-emulator/utils"
	"log/slog"
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

func (f *FixNumShardResolver) GetShardByShardId(ctx context.Context, shardId TypeShardId) (*Shard, error) {
	if !f.validateShardId(shardId) {
		retErr := fmt.Errorf("GetShardByShardId failed, get invalid TypeShardId=%d", shardId)
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}
	if f.shardList[shardId] == nil {
		retErr := fmt.Errorf("GetShardByShardId failed, shardId=%d not found", shardId)
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}
	return f.shardList[shardId], nil
}

func (f *FixNumShardResolver) GetNodeIdsInShard(ctx context.Context, shardId TypeShardId) ([]TypeNodeId, error) {
	if !f.validateShardId(shardId) {
		retErr := fmt.Errorf("GetNodesInShard failed, get invalid TypeShardId=%d", shardId)
		slog.ErrorContext(ctx, retErr.Error())
		return nil, retErr
	}

	nodeIds := f.shardNodeBiMap.GetByKey(shardId)
	return nodeIds, nil
}

func (f *FixNumShardResolver) AddShard(ctx context.Context, shard Shard) error {
	shardId := shard.Id
	if !f.validateShardId(shardId) {
		retErr := fmt.Errorf("AddShard failed, get invalid TypeShardId=%d", shardId)
		slog.ErrorContext(ctx, retErr.Error())
		return retErr
	}

	if f.shardList[shardId] != nil {
		retErr := fmt.Errorf("AddShard failed, TypeShardId %d already exists", shardId)
		slog.ErrorContext(ctx, retErr.Error())
		return retErr
	}

	f.shardList[shardId] = &shard
	return nil
}

func (f *FixNumShardResolver) CloseShard(ctx context.Context, shardId TypeShardId) error {
	if !f.validateShardId(shardId) {
		retErr := fmt.Errorf("CloseShard failed, get invalid TypeShardId=%d", shardId)
		slog.ErrorContext(ctx, retErr.Error())
		return retErr
	}

	f.shardList[shardId] = nil
	f.shardNodeBiMap.RemoveByKey(shardId)
	slog.InfoContext(ctx, fmt.Sprintf("CloseShard success: shardId %d", shardId))
	return nil
}

func (f *FixNumShardResolver) GetNodeByNodeId(ctx context.Context, nodeId TypeNodeId) (*Node, error) {
	if node, ok := f.nodeId2Node[nodeId]; ok {
		return node, nil
	}
	retErr := fmt.Errorf("GetNodeByNodeId TypeNodeId %d not found", nodeId)
	slog.ErrorContext(ctx, retErr.Error())
	return nil, retErr
}

func (f *FixNumShardResolver) GetLocShardIdsByNodeId(ctx context.Context, nodeId TypeNodeId) ([]TypeShardId, error) {
	shardIds := f.shardNodeBiMap.GetByValue(nodeId)
	return shardIds, nil
}

func (f *FixNumShardResolver) AddNodeToShard(ctx context.Context, node Node, destShardId TypeShardId) error {
	if !f.validateShardId(destShardId) {
		retErr := fmt.Errorf("AddNodeToShard failed, get invalid TypeShardId=%d", destShardId)
		slog.ErrorContext(ctx, retErr.Error())
		return retErr
	}
	if _, exist := f.nodeId2Node[node.Id]; exist {
		retErr := fmt.Errorf("AddNodeToShard failed, TypeShardId %d already exists", node.Id)
		slog.ErrorContext(ctx, retErr.Error())
		return retErr
	}
	f.nodeId2Node[node.Id] = &node
	f.shardNodeBiMap.Add(destShardId, node.Id)
	return nil
}

func (f *FixNumShardResolver) DeleteNode(ctx context.Context, nodeId TypeNodeId) error {
	f.nodeId2Node[nodeId] = nil
	f.shardNodeBiMap.RemoveByValue(nodeId)
	slog.InfoContext(ctx, fmt.Sprintf("DeleteNode success: nodeId %d", nodeId))
	return nil
}

func (f *FixNumShardResolver) GetLocShardsIdByAccountAddr(ctx context.Context, addr account.Address) ([]TypeShardId, error) {
	shardIds := f.shardAccountBiMap.GetByValue(addr)
	if len(shardIds) == 0 {
		return []TypeShardId{f.getDefaultShard(addr)}, nil
	}
	return shardIds, nil
}

func (f *FixNumShardResolver) AddAccountToShard(ctx context.Context, addr account.Address, destShardId TypeShardId) error {
	if !f.validateShardId(destShardId) {
		retErr := fmt.Errorf("AddAccount failed, get invalid TypeShardId=%d", destShardId)
		slog.ErrorContext(ctx, retErr.Error())
		return retErr
	}
	f.shardAccountBiMap.Add(destShardId, addr)
	return nil
}

func (f *FixNumShardResolver) DeleteAccountInShard(ctx context.Context, addr account.Address, shardId TypeShardId) error {
	if !f.validateShardId(shardId) {
		retErr := fmt.Errorf("DeleteAccount failed, get invalid TypeShardId=%d", shardId)
		slog.ErrorContext(ctx, retErr.Error())
		return retErr
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
