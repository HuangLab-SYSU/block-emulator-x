package txsource

import (
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator-x/config"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/supervisor/txsource/csvsource"
	"github.com/HuangLab-SYSU/block-emulator-x/supervisor/txsource/randomsource"
)

// TxSource provides a transaction source for the supervisor (as the client / wallet).
// Transactions will be read from TxSource and sent to the consensus nodes periodically.
type TxSource interface {
	// ReadTxs reads transactions from the TxSource. If the source is exhausted, it returns (nil, nil).
	ReadTxs(size int64) ([]transaction.Transaction, error)
}

type NoOperationTxSource struct{}

func (NoOperationTxSource) ReadTxs(int64) ([]transaction.Transaction, error) {
	return nil, nil
}

// NewTxSource creates a TxSource by the given config.
func NewTxSource(cfg config.TxSourceCfg) (TxSource, error) {
	var ts TxSource

	switch cfg.TxSourceType {
	case csvsource.Key:
		cs, err := csvsource.NewCSVSource(cfg.TxSourceFile, cfg.ExcludeContractTxs)
		if err != nil {
			return nil, fmt.Errorf("failed to create CSV source: %w", err)
		}

		ts = cs
	case randomsource.Key:
		ts = randomsource.NewRandomSource()
	default:
		ts = NoOperationTxSource{}
	}

	return ts, nil
}
