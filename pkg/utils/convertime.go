package utils

import "time"

func ConvertTime2Str(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format(time.RFC3339)
}
