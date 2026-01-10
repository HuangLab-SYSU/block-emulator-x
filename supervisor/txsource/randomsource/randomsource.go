package randomsource

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
)

const (
	Key = "random_source"

	// accountPrefixZeroBytes is the number of zero bytes in the generated random accounts.
	// The smaller accountPrefixZeroBytes is, the randomer the account addresses are.
	accountPrefixZeroBytes = 18
	// upperBoundTransferAmount is the upper bound of transfer number.
	upperBoundTransferAmount = 1000
	// feePercentage is the percentage of transaction priority fee.
	feePercentage = 0.1
)

type RandomSource struct {
	count int64
}

func NewRandomSource() *RandomSource {
	return &RandomSource{
		count: 1,
	}
}

func (r *RandomSource) ReadTxs(size int64) ([]transaction.Transaction, error) {
	txs := make([]transaction.Transaction, size)
	for i := range size {
		txs[i] = generateRandomTransaction(r.count)
		r.count++
	}

	return txs, nil
}

func generateRandomTransaction(c int64) transaction.Transaction {
	var sender, receiver account.Address

	_, _ = rand.Read(sender[accountPrefixZeroBytes:])

	_, _ = rand.Read(receiver[accountPrefixZeroBytes:])
	for sender == receiver {
		_, _ = rand.Read(receiver[accountPrefixZeroBytes:])
	}

	amount, _ := rand.Int(rand.Reader, big.NewInt(upperBoundTransferAmount))
	// The random transfer number should be at least 1.
	amount.Add(amount, big.NewInt(1))
	fee := big.NewInt(int64(float64(amount.Int64()) * feePercentage))

	return *transaction.NewTransaction(sender, receiver, amount, fee, uint64(c), time.Now())
}
