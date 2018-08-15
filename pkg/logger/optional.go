package logger

// Optional is a logger used to make logging optional
type Optional struct {
	logger Logger
}

// NewOptional creates a new Optional logger
func NewOptional(l Logger) *Optional {
	if l == nil {
		return nil
	}
	switch l := l.(type) {
	case *Optional:
		return l
	default:
		return &Optional{logger: l}
	}
}

// WithFields creates a child logger with fields
func (l *Optional) WithFields(fields ...Field) Logger {
	if l == nil {
		return nil
	}
	return NewOptional(l.logger.WithFields(fields...))
}

// Debug logs the lowest logging level
func (l *Optional) Debug(msg string) {
	if l == nil {
		return
	}
	l.logger.Debug(msg)
}
