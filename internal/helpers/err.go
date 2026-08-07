package helpers

import (
	"fmt"
	"runtime"
)

// WrapError adds file and line number information to an error message
func WrapError(message string) error {
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}
	return fmt.Errorf("%s (at %s:%d)", message, file, line)
}
