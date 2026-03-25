package utils

import "fmt"

// Logger provides structured logging.
type Logger struct {
	prefix string
}

// NewLogger creates a new Logger with the given prefix.
func NewLogger(prefix string) *Logger {
	return &Logger{prefix: prefix}
}

// Info logs an informational message.
func (l *Logger) Info(msg string) {
	fmt.Printf("[%s] INFO: %s\n", l.prefix, msg)
}

// Error logs an error message.
func (l *Logger) Error(msg string) {
	fmt.Printf("[%s] ERROR: %s\n", l.prefix, msg)
}
