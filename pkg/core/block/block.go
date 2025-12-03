// Definition of block

package block

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
)

var RecordTitle = []string{
	"ParentHash",
	"BlockHash",
	"StateRoot",
	"Number",
	"CreateTime",
	"TxRoot",
	"TxBodyLen",
	"MigratedAccountsRoot",
	"MigrationAccountLen",
}

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

func ConvertBlock2Line(b *Block) ([]string, error) {
	blockHash, err := b.Hash()
	if err != nil {
		return nil, fmt.Errorf("CalcHash failed: %w", err)
	}

	return []string{
		hex.EncodeToString(b.ParentBlockHash),      // "ParentHash"
		hex.EncodeToString(blockHash),              // "BlockHash"
		hex.EncodeToString(b.StateRoot),            // "StateRoot"
		fmt.Sprintf("%d", b.Number),                // "Number"
		utils.ConvertTime2Str(b.CreateTime),        // "CreateTime"
		hex.EncodeToString(b.TxRoot),               // "TxRoot"
		fmt.Sprintf("%d", len(b.TxList)),           // "TxBodyLen"
		hex.EncodeToString(b.MigratedAccountsRoot), // "MigratedAccountsRoot"
		fmt.Sprintf("%d", len(b.MigratedAccounts)), // "MigrationAccountLen"
	}, nil
}
