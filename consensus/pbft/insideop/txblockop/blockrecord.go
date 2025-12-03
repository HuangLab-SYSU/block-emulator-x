package txblockop

import (
	"fmt"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/block"
	"github.com/HuangLab-SYSU/block-emulator/pkg/csvwrite"
)

const blockRecordPathFmt = "shard=%d_node=%d/block_record.csv"

func recordBlock(caller *csvwrite.CSVSeqWriter, b *block.Block) error {
	line, err := block.ConvertBlock2Line(b)
	if err != nil {
		return fmt.Errorf("ConvertBlock2Line failed: %w", err)
	}

	if err = caller.WriteLine2CSV(line); err != nil {
		return fmt.Errorf("WriteLine2CSV failed: %w", err)
	}

	return nil
}
