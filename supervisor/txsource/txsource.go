package txsource

import (
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/config"
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource/csvsource"
	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource/randomsource"
)

type TxSource interface {
	ReadTxs(size int64) ([]transaction.Transaction, error)
}

type NoOperationTxSource struct{}

func (NoOperationTxSource) ReadTxs(int64) ([]transaction.Transaction, error) {
	return nil, nil
}

func NewTxSource(cfg config.TxSourceCfg) (TxSource, error) {
	var ts TxSource

	switch cfg.TxSourceType {
	case csvsource.Key:
		cs, err := csvsource.NewCSVSource(cfg.TxSourceFile)
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
