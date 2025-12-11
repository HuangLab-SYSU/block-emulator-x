package logger

import (
	"log/slog"
	"runtime"
	"time"
)

const resourceLogInterval = 3 * time.Second

func resourceLog() {
	var m runtime.MemStats

	tt := time.NewTicker(resourceLogInterval)
	for range tt.C {
		runtime.ReadMemStats(&m)
		slog.Debug("running consensus", "memo used", m.Alloc>>20, "go routines", runtime.NumGoroutine())
	}
}
