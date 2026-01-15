package account

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
)

const initBalanceStr = "1000000000000000000000000000000000000"

var ErrNotEnoughBalance = errors.New("not enough balance")

// State record the details of an account, and it will be saved in the mpt.
// Note that, to meet the compatible to evm, `ShardLocation` will be stored in a single mpt,
// while other will be set to a state db.
type State struct {
	Account     Account
	Nonce       uint64
	Balance     *big.Int
	StorageRoot []byte // storage root of contract structure
	CodeHash    []byte // the code hash of the smart contract account

	// ShardLocation is only used in sharding blockchain.
	// It is to denotes the location of an account.
	// In block-emulator, a new trie is used to store it.
	ShardLocation uint64
}

func NewState(addr Address, loc int64) *State {
	var b big.Int
	b.SetString(initBalanceStr, 10)

	return &State{
		Account: Account{Addr: addr},
		Balance: &b,

		ShardLocation: uint64(loc),
	}
}

// Credit increase the balance of an account.
func (s *State) Credit(value *big.Int) {
	s.Balance.Add(s.Balance, value)
}

// Debit reduce the balance of an account.
func (s *State) Debit(val *big.Int) error {
	if s.Balance.Cmp(val) < 0 {
		return ErrNotEnoughBalance
	}

	s.Balance.Sub(s.Balance, val)

	return nil
}

// Encode encodes states using gob.
func (s *State) Encode() ([]byte, error) {
	var buff bytes.Buffer

	encoder := gob.NewEncoder(&buff)

	err := encoder.Encode(s)
	if err != nil {
		return nil, fmt.Errorf("encode state failed: %w", err)
	}

	return buff.Bytes(), nil
}

// DecodeState decodes states using gob.
func DecodeState(b []byte) (*State, error) {
	var s State

	decoder := gob.NewDecoder(bytes.NewReader(b))

	err := decoder.Decode(&s)
	if err != nil {
		return nil, fmt.Errorf("decode state failed: %w", err)
	}

	return &s, nil
}
