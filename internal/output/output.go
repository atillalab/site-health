package output

import (
	"fmt"
	"os"
)

var (
	Green  = "\033[32m"
	Red    = "\033[31m"
	Yellow = "\033[33m"
	Gray   = "\033[90m"
	Bold   = "\033[1m"
	Reset  = "\033[0m"
)

func init() {
	if !IsTerminal(os.Stdout) {
		Green = ""
		Red = ""
		Yellow = ""
		Gray = ""
		Bold = ""
		Reset = ""
	}
}

// IsTerminal reports whether f is a character device (terminal).
func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// StatusColor returns the ANSI color sequence for a status token.
func StatusColor(status string) string {
	switch status {
	case "OK", "HEALTHY":
		return Green
	case "WARN", "WARNING":
		return Yellow
	case "FAIL", "UNHEALTHY":
		return Red
	case "SKIP":
		return Gray
	default:
		return Reset
	}
}

// FormatStatusToken returns a status string wrapped in its status color.
func FormatStatusToken(status string) string {
	return fmt.Sprintf("%s%s%s", StatusColor(status), status, Reset)
}
