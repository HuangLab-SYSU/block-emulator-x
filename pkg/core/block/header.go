package block

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/bloom"
)

type Header struct {
	ParentBlockHash []byte
	StateRoot       []byte
	TxRoot          []byte
	Bloom           bloom.Filter
	Number          int64
	Miner           account.Address
	CreateTime      time.Time
}

// Encode encodes blockHeaders.
func (h *Header) Encode() ([]byte, error) {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)

	err := enc.Encode(h)
	if err != nil {
		return nil, fmt.Errorf("encode header failed: %w", err)
	}

	return buff.Bytes(), nil
}

// DecodeBlockHeader decodes blockHeaders.
func (h *Header) DecodeBlockHeader(b []byte) (*Header, error) {
	var blockHeader Header

	decoder := gob.NewDecoder(bytes.NewReader(b))

	err := decoder.Decode(&blockHeader)
	if err != nil {
		return nil, fmt.Errorf("decode header failed: %w", err)
	}

	return &blockHeader, nil
}
