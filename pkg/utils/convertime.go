package utils

import "time"

// ConvertTime2Str converts time to string.
// If the time is zero, an empty string will be returned.
func ConvertTime2Str(t time.Time) string {
	if t.IsZero() {
		return ""
	}

	return t.Format(time.RFC3339)
}
