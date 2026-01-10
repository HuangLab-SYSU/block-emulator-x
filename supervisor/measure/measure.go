package measure

import "github.com/HuangLab-SYSU/block-emulator-x/pkg/network/rpcserver"

// Measure provides the methods to record and calculate the metrics of a blockchain system.
type Measure interface {
	// UpdateMeasureRecord updates this measure implementation by the WrappedMsg
	UpdateMeasureRecord(msg *rpcserver.WrappedMsg) error
	// OutputResultAndClose output the final result to the csv files.
	OutputResultAndClose() error
}
