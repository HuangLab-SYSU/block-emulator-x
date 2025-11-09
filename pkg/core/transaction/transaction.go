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

const (
	RawTxBrokerStage  = 0
	Sigma1BrokerStage = 1
	Sigma2BrokerStage = 2
)

type Signature []byte

type Transaction struct {
	Sender     account.Account
	Recipient  account.Account
	Value      *big.Int
	Nonce      int64
	Signature  Signature
	CreateTime time.Time

	RelayTxOpt
	BrokerTxOpt
}

type RelayTxOpt struct {
	RelayStage    int
	ROriginalHash []byte
}

type BrokerTxOpt struct {
	BrokerStage               int // label that this is a sigma_1 tx or a sigma_2 tx.
	BrokerAddr                account.Address
	BOriginalHash             []byte // the hash of raw message
	OriginalTxCreateTime      time.Time
	NonceBroker               int64
	HeightLock, HeightCurrent int64
}

func NewTransaction(sender, recipient account.Account, value *big.Int, nonce int64, proposeTime time.Time) *Transaction {
	tx := &Transaction{
		Sender:     sender,
		Recipient:  recipient,
		Value:      value,
		Nonce:      nonce,
		CreateTime: proposeTime,
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
