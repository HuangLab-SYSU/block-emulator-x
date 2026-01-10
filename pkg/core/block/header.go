package block

import (
	"crypto/sha256"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/bloom"
)

type Header struct {
	ParentBlockHash []byte
	StateRoot       []byte
	Number          uint64
	Type            uint8
	Miner           account.Address
	CreateTime      time.Time

	TxHeaderOpt
	MigrationHeaderOpt
}

// TxHeaderOpt is the struct for transaction handling.
// This struct should be used when this block is a normal one (not a block for account migration).
type TxHeaderOpt struct {
	TxRoot []byte
	Bloom  bloom.Filter
}

// MigrationHeaderOpt is the struct for the account migration.
// This struct should be used when this block is an account migration one.
type MigrationHeaderOpt struct {
	MigratedAccountsRoot []byte // MigratedAccountsRoot is the Merkle root of MigratedAccounts in MigrationOpt.
}

// Encode encodes blockHeaders.
// Note that, gob is not useful here.
func (h Header) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(h)
}

func (h Header) Hash() ([]byte, error) {
	b, err := h.Encode()
	if err != nil {
		return []byte{}, err
	}

	sum := sha256.Sum256(b)

	return sum[:], nil
}
