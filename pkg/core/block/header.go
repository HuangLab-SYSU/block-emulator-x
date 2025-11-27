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
	Number          int64
	Miner           account.Address
	CreateTime      time.Time

	TxHeaderOpt
	MigrationHeaderOpt
}

// TxHeaderOpt is the struct for transaction handling.
// This struct should be used when this block is a normal one (not a block for account migration).
// Note that, either the variables in TxHeaderOpt or those in MigrationHeaderOpt is nil.
type TxHeaderOpt struct {
	TxRoot []byte
	Bloom  bloom.Filter
}

// MigrationHeaderOpt is the struct for the account migration.
// This struct should be used when this block is account migration.
// Note that, either the variables in MigrationHeaderOpt or those in TxHeaderOpt is nil.
type MigrationHeaderOpt struct {
	MigratedAccountsRoot []byte // MigratedAccountsRoot is the merkle root of MigratedAccounts in MigrationOpt.
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
