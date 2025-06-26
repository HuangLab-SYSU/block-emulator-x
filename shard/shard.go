package shard

import (
	"context"
	"github.com/HuangLab-SYSU/block-emulator/core/account"
)

type TypeShardId int64
type TypeNodeId int64

type Shard struct {
	Id   TypeShardId
	Name string
}

type Node struct {
	Id     TypeNodeId
	Name   string
	IPAddr string
}

type Resolver interface {
	GetShardByShardId(ctx context.Context, shardId TypeShardId) (*Shard, error)
	GetNodeIdsInShard(ctx context.Context, shardId TypeShardId) ([]TypeNodeId, error)
	AddShard(ctx context.Context, shard Shard) error
	CloseShard(ctx context.Context, shardId TypeShardId) error

	GetNodeByNodeId(ctx context.Context, nodeId TypeNodeId) (*Node, error)
	GetLocShardIdsByNodeId(ctx context.Context, nodeId TypeNodeId) ([]TypeShardId, error)
	AddNodeToShard(ctx context.Context, node Node, destShardId TypeShardId) error
	DeleteNode(ctx context.Context, nodeId TypeNodeId) error

	GetLocShardsIdByAccountAddr(ctx context.Context, addr account.Address) ([]TypeShardId, error)
	AddAccountToShard(ctx context.Context, addr account.Address, destShardId TypeShardId) error
	DeleteAccountInShard(ctx context.Context, addr account.Address, shardId TypeShardId) error
}
