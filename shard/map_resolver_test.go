package shard

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/HuangLab-SYSU/block-emulator/core/account"
)

func TestFixNumberMapResolver(t *testing.T) {
	ctx := context.Background()
	resolver, err := NewFixNumShardResolver(3)
	assert.NoError(t, err)

	// ===== Shard Tests =====
	sh := Shard{Id: 0, Name: "Shard-0"}
	err = resolver.AddShard(ctx, sh)
	assert.NoError(t, err)

	gotShard, err := resolver.GetShardByShardId(ctx, 0)
	assert.NoError(t, err)
	assert.Equal(t, sh.Name, gotShard.Name)

	err = resolver.AddShard(ctx, Shard{Id: 0, Name: "Shard-0-again"})
	assert.Error(t, err)

	err = resolver.AddShard(ctx, Shard{Id: -1})
	assert.Error(t, err)

	err = resolver.AddShard(ctx, Shard{Id: 1, Name: "Shard-1"})
	assert.NoError(t, err)
	err = resolver.CloseShard(ctx, 1)
	assert.NoError(t, err)

	shClosed, err := resolver.GetShardByShardId(ctx, 1)
	assert.Error(t, err)
	assert.Nil(t, shClosed)

	// ===== Node Tests =====
	n := Node{Id: 101, Name: "Node-1", IPAddr: "127.0.0.1"}
	err = resolver.AddNodeToShard(ctx, n, 0)
	assert.NoError(t, err)

	gotNode, err := resolver.GetNodeByNodeId(ctx, 101)
	assert.NoError(t, err)
	assert.Equal(t, n.IPAddr, gotNode.IPAddr)

	nids, _ := resolver.GetNodeIdsInShard(ctx, 0)
	assert.Contains(t, nids, TypeNodeId(101))

	shardsForNode, _ := resolver.GetLocShardIdsByNodeId(ctx, 101)
	assert.Contains(t, shardsForNode, TypeShardId(0))

	err = resolver.AddNodeToShard(ctx, n, 0)
	assert.Error(t, err)

	err = resolver.DeleteNode(ctx, 101)
	assert.NoError(t, err)
	gotNode, _ = resolver.GetNodeByNodeId(ctx, 101)
	assert.Nil(t, gotNode)

	// ===== Account Tests =====
	addr := account.Address{'a', 'b', 'c', 'd', 'e'}
	err = resolver.AddAccountToShard(ctx, addr, 2)
	assert.NoError(t, err)

	shards, err := resolver.GetLocShardsIdByAccountAddr(ctx, addr)
	assert.NoError(t, err)
	assert.Contains(t, shards, TypeShardId(2))

	err = resolver.DeleteAccountInShard(ctx, addr, 2)
	assert.NoError(t, err)

	shards, err = resolver.GetLocShardsIdByAccountAddr(ctx, addr)
	assert.NoError(t, err)
	assert.NotEmpty(t, shards) // default shard fallback

	err = resolver.AddAccountToShard(ctx, addr, 999)
	assert.Error(t, err)

	// ===== Multi Shard/Node/Account Test =====
	err = resolver.AddShard(ctx, Shard{Id: 2, Name: "Shard-2"})
	assert.NoError(t, err)

	accounts := []account.Address{
		{'a', 'b', 'c', 'd', 'e', '1'},
		{'a', 'b', 'c', 'd', 'e', '2'},
		{'a', 'b', 'c', 'd', 'e', '3'},
	}
	nodes := []Node{
		{Id: 201, Name: "Node-A", IPAddr: "10.0.0.1"},
		{Id: 202, Name: "Node-B", IPAddr: "10.0.0.2"},
		{Id: 203, Name: "Node-C", IPAddr: "10.0.0.3"},
	}

	// allocated nodes and accounts to other nodes
	for i, n := range nodes {
		err := resolver.AddNodeToShard(ctx, n, TypeShardId(i%3))
		assert.NoError(t, err)
	}
	for i, acc := range accounts {
		err := resolver.AddAccountToShard(ctx, acc, TypeShardId(i%3))
		assert.NoError(t, err)
	}

	// check reversal search
	for i, n := range nodes {
		shards, err := resolver.GetLocShardIdsByNodeId(ctx, n.Id)
		assert.NoError(t, err)
		assert.Contains(t, shards, TypeShardId(i%3))
	}
	for i, acc := range accounts {
		shards, err := resolver.GetLocShardsIdByAccountAddr(ctx, acc)
		assert.NoError(t, err)
		assert.Contains(t, shards, TypeShardId(i%3))
	}
}
