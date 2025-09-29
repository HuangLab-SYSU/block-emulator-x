package block

import (
	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
)

type Body struct {
	TxList []transaction.Transaction
}
