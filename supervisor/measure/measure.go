package measure

import "github.com/HuangLab-SYSU/block-emulator/pkg/network/rpcserver"

type Measure interface {
	// UpdateMeasureRecord updates this measure implementation by the WrappedMsg
	UpdateMeasureRecord(msg *rpcserver.WrappedMsg) error
	// OutputResultAndClose output the final result to the csv files.
	OutputResultAndClose() error
}
