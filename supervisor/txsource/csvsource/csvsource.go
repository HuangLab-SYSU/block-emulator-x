package csvsource

import (
	"encoding/csv"
	"fmt"
	"io"
	"math/big"
	"os"
	"time"

	"github.com/HuangLab-SYSU/block-emulator/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator/pkg/utils"
)

const Key = "csv_source"

// CSVSource implements TxSourceType.
// The csv file format supported by this implementation is like those from XBlock (https://xblock.pro/xblock-eth.html).
type CSVSource struct {
	count int64
	cr    *csv.Reader
	file  *os.File
	done  bool
}

func NewCSVSource(filename string) (*CSVSource, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open dataset file: %w", err)
	}

	r := csv.NewReader(f)

	// skip the first line
	_, err = r.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read the first line: %w", err)
	}

	return &CSVSource{
		count: 1,
		cr:    r,
		file:  f,
	}, nil
}

func (ds *CSVSource) ReadTxs(size int64) ([]transaction.Transaction, error) {
	if ds.done {
		return nil, nil
	}

	ret := make([]transaction.Transaction, 0, size)
	for range size {
		txLine, err := ds.cr.Read()
		if err == io.EOF {
			ds.close()
			break
		}

		if err != nil {
			ds.close()
			return nil, fmt.Errorf("failed to read dataset file: %w", err)
		}

		tx, err := line2Tx(txLine, ds.count)
		if err != nil {
			ds.close()
			return nil, fmt.Errorf("failed to transfer line to tx: %w", err)
		}

		ret = append(ret, *tx)
	}

	return ret, nil
}

func (ds *CSVSource) close() {
	ds.done = true
	_ = ds.file.Close()
}

func line2Tx(line []string, count int64) (*transaction.Transaction, error) {
	if line[6] != "0" || line[7] != "0" || line[3] == line[4] {
		return nil, fmt.Errorf("invalid line %v", line)
	}

	val := new(big.Int)
	if _, ok := val.SetString(line[8], 10); !ok {
		return nil, fmt.Errorf("failed to parse value, val=%s", line[8])
	}

	senderAddr, err := utils.Hex2Addr(line[3])
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender address: %w", err)
	}

	receiverAddr, err := utils.Hex2Addr(line[4])
	if err != nil {
		return nil, fmt.Errorf("failed to parse receiver address: %w", err)
	}

	tx := transaction.NewTransaction(senderAddr, receiverAddr, val, uint64(count), time.Now())

	return tx, nil
}
