// Definition of block

package block

import (
	"bytes"
	"encoding/gob"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
)

type Block struct {
	Header *Header
	Body
	MigrationOpt
}

// MigrationOpt is the struct for account migration.
// It saves the information of accounts which are to migrated to this shard.
// Note that, either MigrationOpt or Body is nil.
type MigrationOpt struct {
	MigratedAccounts []account.Account // MigratedAccounts is the list of accounts to be migrated in this stage.
	MigratedStates   []account.State   // MigratedStates is the list of account states corresponding to accounts in MigratedAccounts.
}

// NewBlock creates the normal block with header and body.
func NewBlock(h *Header, b Body) *Block {
	return &Block{Header: h, Body: b}
}

// NewMigrationBlock creates the block for account migration, with header and MigrationOpt.
func NewMigrationBlock(h *Header, opt MigrationOpt) *Block {
	return &Block{Header: h, MigrationOpt: opt}
}

// Encode encodes Block.
func (b *Block) Encode() ([]byte, error) {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)

	err := enc.Encode(b)
	if err != nil {
		return nil, fmt.Errorf("encode block failed: %w", err)
	}

	return buff.Bytes(), nil
}

// DecodeBlock decodes blocks.
func DecodeBlock(b []byte) (*Block, error) {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(b))

	err := decoder.Decode(&block)
	if err != nil {
		return nil, fmt.Errorf("decode block failed: %w", err)
	}

	return &block, nil
}
