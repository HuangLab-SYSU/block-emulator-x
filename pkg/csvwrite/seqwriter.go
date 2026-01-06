package csvwrite

import (
	"encoding/csv"
	"fmt"
	"os"
)

// CSVSeqWriter is a writer to write a csv file sequentially.
// It writes a line by calling WriteLine2CSV and should be closed by calling Close.
type CSVSeqWriter struct {
	csvW *csv.Writer
	file *os.File
}

func NewCSVSeqWriter(path string, title []string) (*CSVSeqWriter, error) {
	f, err := createFileWithDirs(path)
	if err != nil {
		return nil, fmt.Errorf("createFileWithDirs failed: %w", err)
	}

	csvW := csv.NewWriter(f)
	if err = csvW.Write(title); err != nil {
		return nil, fmt.Errorf("csv write failed: %w", err)
	}

	csvW.Flush()

	if err = csvW.Error(); err != nil {
		return nil, fmt.Errorf("csv flush failed: %w", err)
	}

	return &CSVSeqWriter{csvW: csvW, file: f}, nil
}

func (cc *CSVSeqWriter) WriteLine2CSV(line []string) error {
	if err := cc.csvW.Write(line); err != nil {
		return fmt.Errorf("failed to write line: %w", err)
	}

	cc.csvW.Flush()

	return cc.csvW.Error()
}

func (cc *CSVSeqWriter) Close() error {
	return cc.file.Close()
}
