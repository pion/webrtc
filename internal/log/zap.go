package log

import "go.uber.org/zap"

// Zap is a Logger based on go.uber.org/zap
type Zap struct {
	logger *zap.Logger
}

// NewZap creates a Logger from an zap.Logger.
func NewZap(logger *zap.Logger) *Zap {
	return &Zap{logger: logger}
}

// WithFields creates a new logger with fields
func (l *Zap) WithFields(fields ...Field) Logger {
	var zapFields []zap.Field
	for _, field := range fields {
		zapFields = append(zapFields, zap.String(field.Key, field.Value))
	}
	return NewZap(l.logger.With(zapFields...))
}

// Debug logs a debug message
func (l *Zap) Debug(msg string) {
	l.logger.Debug(msg)
}
