// Definition of transaction

package transaction

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math/big"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
)

type Signature []byte

type Transaction struct {
	Sender       account.Account
	Recipient    account.Account
	Value        *big.Int
	Nonce        int64
	Signature    Signature
	ProposedTime time.Time

	BrokerTxOpt
}

type BrokerTxOpt struct {
	FirstBroker                       bool
	OriginalHash                      []byte
	OriginalSender, OriginalRecipient account.Account
}

func NewTransaction(sender, recipient account.Account, value *big.Int, nonce int64, proposeTime time.Time) *Transaction {
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
		return nil, fmt.Errorf("encode transaction err: %w", err)
	}

	return buff.Bytes(), nil
}

// DecodeTx decodes transaction.
func DecodeTx(b []byte) (*Transaction, error) {
	var tx Transaction

	decoder := gob.NewDecoder(bytes.NewReader(b))

	err := decoder.Decode(&tx)
	if err != nil {
		return nil, fmt.Errorf("decode transaction err: %w", err)
	}

	return &tx, nil
}
