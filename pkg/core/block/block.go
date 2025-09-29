// Definition of block

package block

import (
	"bytes"
	"encoding/gob"
	"fmt"
)

type Block struct {
	Header *Header
	Body   *Body
}

func NewBlock(h *Header, b *Body) *Block {
	return &Block{Header: h, Body: b}
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
