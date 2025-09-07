package trie

import (
	"context"
	"testing"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/stretchr/testify/assert"
)

func TestEthereumDefaultTrieBasicFlow(t *testing.T) {
	ctx := context.Background()

	// 构造内存 trie DB
	tdb, err := NewEthereumDefaultTrieDB(&config.EthStorageCfg{IsMemoryDB: true}, nil)
	assert.NoError(t, err)

	// 初始 root 应该是空的
	root, err := tdb.GetCurrentRoot(ctx)
	assert.NoError(t, err)
	assert.Equal(t, types.EmptyRootHash.Bytes(), root)

	// 添加键值并预览 root
	keys := [][]byte{[]byte("alice"), []byte("bob")}
	vals := [][]byte{[]byte("100"), []byte("200")}

	previewRoot, err := tdb.MAddAccountStatesPreview(ctx, keys, vals)
	assert.NoError(t, err)
	assert.NotEqual(t, types.EmptyRootHash.Bytes(), previewRoot)

	// preview 不添加节点
	values, err := tdb.MGetAccountStates(ctx, keys)
	assert.NoError(t, err)
	assert.Empty(t, values[0])
	assert.Empty(t, values[1])

	notCommitRoot, err := tdb.GetCurrentRoot(ctx)
	assert.NoError(t, err)
	assert.Equal(t, types.EmptyRootHash.Bytes(), notCommitRoot)

	// 添加键值并 commit
	committedRoot, err := tdb.MAddAccountStatesAndCommit(ctx, keys, vals)
	assert.NoError(t, err)
	assert.Equal(t, previewRoot, committedRoot)

	// 获取状态值
	retrieved, err := tdb.MGetAccountStates(ctx, keys)
	assert.NoError(t, err)
	assert.Equal(t, vals[0], retrieved[0])
	assert.Equal(t, vals[1], retrieved[1])
}

func TestEthereumDefaultTrieEmptyAndMismatchInputs(t *testing.T) {
	// 构造内存 trie DB
	tdb, err := NewEthereumDefaultTrieDB(&config.EthStorageCfg{IsMemoryDB: true}, nil)
	assert.NoError(t, err)
	ctx := context.Background()

	// 空 key/value
	root, err := tdb.GenerateRootByGivenBytes(ctx, [][]byte{}, [][]byte{})
	assert.NoError(t, err)
	assert.Equal(t, types.EmptyRootHash.Bytes(), root)

	// 长度不一致
	_, err = tdb.GenerateRootByGivenBytes(ctx, [][]byte{[]byte("a")}, [][]byte{})
	assert.Error(t, err)
	_, err = tdb.MAddAccountStatesAndCommit(ctx, [][]byte{[]byte("a")}, [][]byte{})
	assert.Error(t, err)
	_, err = tdb.MAddAccountStatesPreview(ctx, [][]byte{[]byte("a")}, [][]byte{})
	assert.Error(t, err)
}

func TestEthereumDefaultTrieGetUnknownKey(t *testing.T) {
	// 构造内存 trie DB
	tdb, err := NewEthereumDefaultTrieDB(&config.EthStorageCfg{IsMemoryDB: true}, nil)
	assert.NoError(t, err)
	ctx := context.Background()
	vals, err := tdb.MGetAccountStates(ctx, [][]byte{[]byte("unknown")})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(vals))
	assert.Nil(t, vals[0])
}
