package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/HuangLab-SYSU/block-emulator/config"
)

const (
	logFilePathFmt   = "logs/shard=%d_node=%d/%s.log"
	nodeInfoPrintFmt = "S%dN%d"
	levelDebug       = "debug"
	levelInfo        = "info"
	levelWarn        = "warn"
	levelError       = "error"
)

var logFile *os.File = nil

func InitLogger(lp *config.LocalParams, cfg config.LogCfg) error {
	if err := setLogFile(lp, cfg); err != nil {
		return fmt.Errorf("set the logger file failed: %v", err)
	}

	slogLevel := getSlogLevel(cfg)

	var writers []io.Writer

	writers = append(writers, os.Stdout)

	if logFile != nil {
		writers = append(writers, logFile)
	}

	mw := io.MultiWriter(writers...)

	handler := slog.NewTextHandler(mw, &slog.HandlerOptions{Level: slogLevel})
	slogger := slog.New(handler).With("NodeInfo", fmt.Sprintf(nodeInfoPrintFmt, lp.ShardID, lp.NodeID))
	slog.SetDefault(slogger)

	return nil
}

func CloseLoggerFile() {
	if logFile == nil {
		return
	}

	_ = logFile.Close()
}

func setLogFile(lp *config.LocalParams, cfg config.LogCfg) error {
	if cfg.LogDir == "" {
		return nil
	}

	logFp := filepath.Join(cfg.LogDir, fmt.Sprintf(logFilePathFmt, lp.ShardID, lp.NodeID, cfg.LogLevel))

	err := os.MkdirAll(filepath.Dir(logFp), os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	logFile, err = os.OpenFile(logFp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}

	return nil
}

func getSlogLevel(cfg config.LogCfg) slog.Level {
	var slogLevel slog.Level

	switch cfg.LogLevel {
	case levelDebug:
		slogLevel = slog.LevelDebug
	case levelInfo:
		slogLevel = slog.LevelInfo
	case levelWarn:
		slogLevel = slog.LevelWarn
	case levelError:
		slogLevel = slog.LevelError
	default:
		slogLevel = slog.LevelInfo
	}

	return slogLevel
}
