// Definition of block

package block

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

type Block struct {
	Header
	Body
	MigrationOpt
}

// NewBlock creates the normal block with header and body.
func NewBlock(h Header, b Body) *Block {
	return &Block{Header: h, Body: b}
}

// NewMigrationBlock creates the block for account migration, with header and MigrationOpt.
func NewMigrationBlock(h Header, opt MigrationOpt) *Block {
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

func DecodeBlock(data []byte) (*Block, error) {
	var block Block

	err := gob.NewDecoder(bytes.NewReader(data)).Decode(&block)
	if err != nil {
		return nil, fmt.Errorf("decode block failed: %w", err)
	}

	return &block, nil
}
