package block

import "github.com/HuangLab-SYSU/block-emulator/core/transaction"

type Body struct {
	TxList []transaction.Transaction
}
