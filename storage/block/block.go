package block

import (
	"context"
)

// Store should be a key-value database that stores the
// information of blocks.
type Store interface {
	UpdateNewestBlockHash(ctx context.Context, newBlockHash []byte) error
	GetNewestBlockHash(ctx context.Context) ([]byte, error)

	AddBlock(ctx context.Context, blockHash, encodedBlock []byte) error
	GetBlockByHash(ctx context.Context, blockHash []byte) ([]byte, error)

	AddBlockHeader(ctx context.Context, blockHash, encodedBlockHeader []byte) error
	GetBlockHeaderByHash(ctx context.Context, blockHash []byte) ([]byte, error)
}
