package randomsource

import (
	"crypto/rand"
	"math/big"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

const (
	Key = "random_source"

	// accountPrefixZeroBytes is the number of zero bytes in the generated random accounts.
	// The smaller accountPrefixZeroBytes is, the randomer the account addresses are.
	accountPrefixZeroBytes = 18
	// upperBoundTransferAmount is the upper bound of transfer number.
	upperBoundTransferAmount = 1000
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
	var sender, receiver account.Account

	_, _ = rand.Read(sender.Addr[accountPrefixZeroBytes:])

	_, _ = rand.Read(receiver.Addr[accountPrefixZeroBytes:])
	for sender.Addr == receiver.Addr {
		_, _ = rand.Read(receiver.Addr[accountPrefixZeroBytes:])
	}

	amount, _ := rand.Int(rand.Reader, big.NewInt(upperBoundTransferAmount))
	// The random transfer number should be at least 1.
	amount.Add(amount, big.NewInt(1))

	return *transaction.NewTransaction(sender, receiver, amount, uint64(c), time.Now())
}
