package csvsource

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/account"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/core/transaction"
	"github.com/HuangLab-SYSU/block-emulator-x/pkg/utils"
)

const Key = "csv_source"

// CSVSource implements TxSourceType.
// The csv file format supported by this implementation is like those from XBlock (https://xblock.pro/xblock-eth.html).
type CSVSource struct {
	count              int64
	cr                 *csv.Reader
	file               *os.File
	done               bool
	excludeContractTxs bool
}

func NewCSVSource(filename string, excludeContractTxs bool) (*CSVSource, error) {
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
		count:              1,
		cr:                 r,
		file:               f,
		excludeContractTxs: excludeContractTxs,
	}, nil
}

func (ds *CSVSource) ReadTxs(size int64) ([]transaction.Transaction, error) {
	if ds.done {
		return nil, nil
	}

	ret := make([]transaction.Transaction, 0, size)
	for int64(len(ret)) < size {
		txLine, err := ds.cr.Read()
		if errors.Is(err, io.EOF) {
			ds.close()
			break
		}

		if err != nil {
			ds.close()
			return nil, fmt.Errorf("failed to read dataset file: %w", err)
		}

		tx, err := line2Tx(txLine, ds.count, ds.excludeContractTxs)
		if err != nil {
			slog.Debug("line2Tx failed", "line", txLine, "err", err)
			continue
		}

		ds.count++

		ret = append(ret, *tx)
	}

	return ret, nil
}

func (ds *CSVSource) close() {
	ds.done = true
	_ = ds.file.Close()
}

func line2Tx(line []string, count int64, excludeContractTxs bool) (*transaction.Transaction, error) {
	fromAddrStr := line[3]
	toAddrStr := line[4]
	toCreateStr := line[5]
	callingFuncStr := line[12]

	hasCallData := callingFuncStr != "" && !strings.EqualFold(callingFuncStr, "none") &&
		!strings.EqualFold(callingFuncStr, "0x")
	hasToCreate := toCreateStr != "" && !strings.EqualFold(toCreateStr, "none")
	hasToAddr := toAddrStr != "" && !strings.EqualFold(toAddrStr, "none")
	fromIsContract := line[6] == "1"
	toIsContract := line[7] == "1"

	isContractRelated := (!hasToAddr && hasToCreate) || fromIsContract || toIsContract

	if isContractRelated && excludeContractTxs {
		return nil, fmt.Errorf("contract transactions have been filtered")
	}

	if hasToAddr && fromAddrStr == toAddrStr {
		return nil, fmt.Errorf("sender and recipient are the same")
	}

	val := new(big.Int)
	if _, ok := val.SetString(line[8], 10); !ok {
		return nil, fmt.Errorf("failed to parse value, val=%s", line[8])
	}

	senderAddr, err := utils.Hex2Addr(fromAddrStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse sender address: %w", err)
	}

	receiverAddr := account.EmptyAccountAddr
	if hasToAddr {
		receiverAddr, err = utils.Hex2Addr(toAddrStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse receiver address: %w", err)
		}
	} else if !hasToCreate {
		return nil, fmt.Errorf("missing account address")
	}

	tx := transaction.NewTransaction(senderAddr, receiverAddr, val, big.NewInt(0), uint64(count), time.Now())

	if hasCallData {
		data, err := utils.Hex2Bytes(callingFuncStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse calling function: %w", err)
		}

		tx.Data = data
	}

	return tx, nil
}
