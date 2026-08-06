package helpers

import "strings"

// ShellQuote wraps s in single quotes, escaping embedded single quotes, so
// arbitrary values survive interpolation into a POSIX shell command.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
