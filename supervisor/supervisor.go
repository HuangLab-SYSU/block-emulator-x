package supervisor

import (
	"log/slog"

	"github.com/HuangLab-SYSU/block-emulator/supervisor/txsource"
)

type Supervisor struct {
	logger   slog.Logger       // logger logs the information.
	txSource txsource.TxSource // txSource brings the txs into the blockchain system.
}

func NewSupervisor(txSource txsource.TxSource) *Supervisor {
	return &Supervisor{
		txSource: txSource,
	}
}
