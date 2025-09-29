package block

import (
	"context"
)

// Store should be a key-value database that stores the
// information of blocks.
type Store interface {
	// AddBlock adds a block into the database. It contains the operations of
	// (1) updating the newest blockHash,
	// (2) adding the header of this block into the storage,
	// (3) adding the block into the storage.
	// These 3 operations must be atomic.
	AddBlock(ctx context.Context, blockHash, encodedBlock, encodedBlockHeader []byte) error
	// GetBlockByHash gets the block according to its Hash.
	GetBlockByHash(ctx context.Context, blockHash []byte) ([]byte, error)

	// AddBlockHeader adds the header of a block into the database. It contains the operations of
	// (1) updating the newest blockHash,
	// (2) adding the header of this block into the storage.
	// These 2 operations must be atomic. Please distinguish it from AddBlock.
	// If your storage is limited, AddBlockHeader helps you catch up with other nodes quickly
	// because it reduces the storage of Block.
	AddBlockHeader(ctx context.Context, blockHash, encodedBlockHeader []byte) error
	// GetBlockHeaderByHash gets the block header according to its blockHash.
	GetBlockHeaderByHash(ctx context.Context, blockHash []byte) ([]byte, error)

	// UpdateNewestBlockHash updates the newest blockHash.
	// This function should be called when the blockchain wants to rollback.
	UpdateNewestBlockHash(ctx context.Context, newBlockHash []byte) error
	// GetNewestBlockHash gets the newest blockHash.
	// Blockchain can quickly find the tail of a chain.
	GetNewestBlockHash(ctx context.Context) ([]byte, error)

	Close() error
}
