// Definition of transaction

package transaction

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"github.com/HuangLab-SYSU/block-emulator/core/account"
	"math/big"
	"time"
)

type Signature []byte

type Transaction struct {
	Sender       account.Address
	Recipient    account.Address
	Value        *big.Int
	Nonce        int64
	Signature    Signature
	ProposedTime time.Time
}

func NewTransaction(sender, recipient account.Address, value *big.Int, nonce int64, proposeTime time.Time) *Transaction {
	tx := &Transaction{
		Sender:       sender,
		Recipient:    recipient,
		Value:        value,
		Nonce:        nonce,
		ProposedTime: proposeTime,
	}
	return tx
}

// Encode encodes transactions.
func (tx *Transaction) Encode() ([]byte, error) {
	var buff bytes.Buffer
	enc := gob.NewEncoder(&buff)
	err := enc.Encode(tx)
	if err != nil {
		return nil, fmt.Errorf("encode transaction err: %v", err)
	}
	return buff.Bytes(), nil
}

// DecodeTx decodes transaction
func DecodeTx(b []byte) (*Transaction, error) {
	var tx Transaction
	decoder := gob.NewDecoder(bytes.NewReader(b))
	err := decoder.Decode(&tx)
	if err != nil {
		return nil, fmt.Errorf("decode transaction err: %v", err)
	}
	return &tx, nil
}
