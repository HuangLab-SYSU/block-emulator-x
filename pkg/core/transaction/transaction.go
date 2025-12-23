// Definition of transaction

package transaction

import (
	"crypto/sha256"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/rlp"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
)

const (
	UndeterminedRelayTx = 0
	Relay1Tx            = 1
	Relay2Tx            = 2

	RawTxBrokerStage  = 0
	Sigma1BrokerStage = 1
	Sigma2BrokerStage = 2
)

type Signature []byte

type Transaction struct {
	Sender     account.Address
	Recipient  account.Address
	Value      *big.Int
	Nonce      uint64
	Signature  Signature
	CreateTime time.Time

	RelayTxOpt  // the optional setting only for relay transactions.
	BrokerTxOpt // the optional setting only for broker transactions.
}

type RelayTxOpt struct {
	RelayStage    uint
	ROriginalHash []byte
}

type BrokerTxOpt struct {
	BrokerStage               uint // label that this is a sigma_1 tx or a sigma_2 tx.
	Broker                    account.Address
	BOriginalHash             []byte // the hash of raw message
	OriginalTxCreateTime      time.Time
	NonceBroker               uint64
	HeightLock, HeightCurrent uint64
}

func NewTransaction(
	sender, recipient account.Address,
	value *big.Int,
	nonce uint64,
	proposeTime time.Time,
) *Transaction {
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
// Transaction encode should be prepare
func (tx *Transaction) Encode() ([]byte, error) {
	return rlp.EncodeToBytes(tx)
}

func (tx *Transaction) Hash() ([]byte, error) {
	b, err := tx.Encode()
	if err != nil {
		return []byte{}, err
	}

	sum := sha256.Sum256(b)

	return sum[:], nil
}
