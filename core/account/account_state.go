package account

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math/big"
)

// State record the details of an account, and it will be saved in the mpt.
type State struct {
	Account     Account
	Nonce       int64
	Balance     *big.Int
	StorageRoot []byte // storage root of contract structure
	CodeHash    []byte // the code hash of the smart contract account
}

// Credit increase the balance of an account
func (s *State) Credit(value *big.Int) {
	s.Balance.Add(s.Balance, value)
}

// Debit reduce the balance of an account
func (s *State) Debit(val *big.Int) error {
	if s.Balance.Cmp(val) < 0 {
		return fmt.Errorf("debit failed: account=%x, balance=%d, debit_value=%d", s.Account.Addr, s.Balance, val)
	}
	s.Balance.Sub(s.Balance, val)
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
