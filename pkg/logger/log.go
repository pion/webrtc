package logger

// Logger is an interface around logging.
// It allows any log package to be plugged in and
// intentionally limits the possible ways to log.
type Logger interface {
	// Debug represents the lowest logging level
	Debug(msg string)

	// WithFields creates a child logger with fields
	WithFields(...Field) Logger
}

// Field is used to add a key-value pair to a logger's context
type Field struct {
	Key   string
	Value string
}
