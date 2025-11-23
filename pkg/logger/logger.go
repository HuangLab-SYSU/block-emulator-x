package logger

import (
	"log/slog"
	"os"
)

func init() {
	InitLogger()
}

func InitLogger() {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})
	slog.SetDefault(slog.New(handler))
}
