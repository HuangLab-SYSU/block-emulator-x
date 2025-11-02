package utils

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func WriteAllToCSV(path string, header []string, rows [][]string) error {
	f, err := createFileWithDirs(path)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}

	defer func() { _ = f.Close() }()

	w := csv.NewWriter(f)

	if err = w.Write(header); err != nil {
		return fmt.Errorf("failed to write header: %w", err)
	}

	for _, row := range rows {
		if err = w.Write(row); err != nil {
			return fmt.Errorf("failed to write row: %w", err)
		}
	}

	w.Flush()

	return w.Error()
}

func createFileWithDirs(path string) (*os.File, error) {
	// create the directory
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	// create the file
	// if the file exists, return error
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil, fmt.Errorf("file already exists: %s", path)
		}

		return nil, err
	}

	return f, nil
}

func ConvertTime2Str(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format(time.RFC3339)
}
