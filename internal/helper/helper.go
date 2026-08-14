package helper

import (
	"fmt"
	"strings"

	"github.com/mattn/go-runewidth"
)

func Width(s string) int { return runewidth.StringWidth(s) }

func PadTo(s string, width int) string {
	if n := Width(s); n < width {
		return s + strings.Repeat(" ", width-n)
	}
	return s
}

func Pad(s string, width int) string {
	return PadTo(s, width)
}

func PadR(s string, width int) string {
	return PadTo(s, width)
}

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
