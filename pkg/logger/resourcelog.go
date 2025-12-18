package logger

import (
	"log/slog"
	"runtime"
	"time"
)

const (
	resourceLogInterval = 3 * time.Second
	byte2MB             = 20
)

func resourceLog() {
	var m runtime.MemStats

	tt := time.NewTicker(resourceLogInterval)
	for range tt.C {
		runtime.ReadMemStats(&m)
		slog.Info("node resources reports", "memo used (MB)", m.Alloc>>byte2MB, "go routines", runtime.NumGoroutine())
	}
}
