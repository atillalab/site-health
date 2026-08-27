package output

import (
	"fmt"
	"os"
)

var (
	green  = "\033[32m"
	red    = "\033[31m"
	yellow = "\033[33m"
	gray   = "\033[90m"
	bold   = "\033[1m"
	reset  = "\033[0m"
)

func init() {
	if !isTerminal(os.Stdout) {
		green = ""
		red = ""
		yellow = ""
		gray = ""
		bold = ""
		reset = ""
	}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func statusColor(status string) string {
	switch status {
	case "OK", "HEALTHY":
		return green
	case "WARN", "WARNING":
		return yellow
	case "FAIL", "UNHEALTHY":
		return red
	case "SKIP":
		return gray
	default:
		return reset
	}
}

func formatStatusToken(status string) string {
	return fmt.Sprintf("%s%s%s", statusColor(status), status, reset)
}
