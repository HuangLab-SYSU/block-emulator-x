package block

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/HuangLab-SYSU/block-emulator/core/account"
	"github.com/HuangLab-SYSU/block-emulator/core/hash"
	"github.com/HuangLab-SYSU/block-emulator/core/mpt"
	"github.com/bits-and-blooms/bitset"
	"time"
)

type Header struct {
	ParentBlockHash hash.Hash
	StateRoot       mpt.NodeKey
	TxRoot          mpt.NodeKey
	Bloom           bitset.BitSet
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
		return nil, fmt.Errorf("encode header failed: %v", err)
	}
	return buff.Bytes(), nil
}

// DecodeBlockHeader decodes blockHeaders.
func (h *Header) DecodeBlockHeader(b []byte) (*Header, error) {
	var blockHeader Header
	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&blockHeader)
	if err != nil {
		return nil, fmt.Errorf("decode header failed: %v", err)
	}
	return &blockHeader, nil
}
