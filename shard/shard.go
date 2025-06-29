package shard

import (
	"context"
	"github.com/HuangLab-SYSU/block-emulator/core/account"
)

type TypeShardId int64
type TypeNodeId int64

// Shard is the basic information of a shard,
// including shardId and shardName.
type Shard struct {
	Id   TypeShardId
	Name string
}

// Node is the basic information of a node,
// including nodeId, nodeName and nodeIpAddr.
type Node struct {
	Id     TypeNodeId
	Name   string
	IPAddr string
}

// Resolver tells about the Shard-Node relation and Shard-Account relation.
type Resolver interface {
	// GetShardByShardId returns the shard with the given shardId.
	GetShardByShardId(ctx context.Context, shardId TypeShardId) (*Shard, error)
	// GetNodeIdsInShard returns the ids of nodes in a shard.
	// A shard consists of some nodes.
	GetNodeIdsInShard(ctx context.Context, shardId TypeShardId) ([]TypeNodeId, error)
	// AddShard adds a shard into the resolver.
	AddShard(ctx context.Context, shard Shard) error
	// CloseShard deletes a shard with the given shardId.
	CloseShard(ctx context.Context, shardId TypeShardId) error

	// GetNodeByNodeId returns the node with the given nodeId.
	GetNodeByNodeId(ctx context.Context, nodeId TypeNodeId) (*Node, error)
	// GetLocShardIdsByNodeId returns the shards which this node is located in.
	// Note that, a node may be located in multiple shards in some mechanisms.
	GetLocShardIdsByNodeId(ctx context.Context, nodeId TypeNodeId) ([]TypeShardId, error)
	// AddNodeToShard adds a node to a shard.
	AddNodeToShard(ctx context.Context, node Node, destShardId TypeShardId) error
	// DeleteNode deletes a node.
	DeleteNode(ctx context.Context, nodeId TypeNodeId) error

	// GetLocShardsIdByAccountAddr returns the shards which this account is located in.
	// It will return a default shard if this account was not located before.
	// Note that, an account may be located in multiple shards,
	// e.g., BrokerChain.
	GetLocShardsIdByAccountAddr(ctx context.Context, addr account.Address) ([]TypeShardId, error)
	// AddAccountToShard adds an account into a shard.
	AddAccountToShard(ctx context.Context, addr account.Address, destShardId TypeShardId) error
	// DeleteAccountInShard deletes the account from the shard.
	DeleteAccountInShard(ctx context.Context, addr account.Address, shardId TypeShardId) error
}
