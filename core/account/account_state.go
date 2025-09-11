package account

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
)

const initBalance = 1_000_000_000

var (
	NotEnoughBalanceErr = errors.New("not enough balance")
)

// State record the details of an account, and it will be saved in the mpt.
type State struct {
	Account        Account
	ShardLocations []int64
	Nonce          int64
	Balance        big.Int
	StorageRoot    []byte // storage root of contract structure
	CodeHash       []byte // the code hash of the smart contract account
}

func NewState(account Account, loc []int64) *State {
	return &State{
		Account:        account,
		ShardLocations: loc,
		Balance:        *big.NewInt(initBalance),
	}
}

// Credit increase the balance of an account
func (s *State) Credit(value *big.Int) {
	s.Balance.Add(&s.Balance, value)
}

// Debit reduce the balance of an account
func (s *State) Debit(val *big.Int) error {
	if s.Balance.Cmp(val) < 0 {
		return NotEnoughBalanceErr
	}
	s.Balance.Sub(&s.Balance, val)
	return nil
}

// Encode encodes states using gob
func (s *State) Encode() ([]byte, error) {
	var buff bytes.Buffer
	encoder := gob.NewEncoder(&buff)
	err := encoder.Encode(s)
	if err != nil {
		return nil, fmt.Errorf("encode state failed: %v", err)
	}
	return buff.Bytes(), nil
}

// DecodeState decodes states using gob
func DecodeState(b []byte) (*State, error) {
	var s State
	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&s)
	if err != nil {
		return nil, fmt.Errorf("decode state failed: %v", err)
	}
	return &s, nil
}
