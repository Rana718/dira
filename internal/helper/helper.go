package helper

import (
	"fmt"
	"strings"
)

// Pad pads s with spaces to width using byte length.
func Pad(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

// PadR pads s using rune count (safe for multi-byte chars like —, ●, …)
func PadR(s string, width int) string {
	r := len([]rune(s))
	if r >= width {
		return s
	}
	return s + strings.Repeat(" ", width-r)
}

// FmtBytes formats bytes into human-readable MB/GB string.
func FmtBytes(b int64) string {
	const (
		MB = 1024 * 1024
		GB = 1024 * MB
	)
	switch {
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/GB)
	case b >= MB:
		return fmt.Sprintf("%.0f MB", float64(b)/MB)
	default:
		return fmt.Sprintf("%d B", b)
	}
}
