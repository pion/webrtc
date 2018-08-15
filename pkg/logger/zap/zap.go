package zap

import (
	"github.com/pions/webrtc/pkg/logger"
	"go.uber.org/zap"
)

// Zap is a Logger based on go.uber.org/zap
type Zap struct {
	logger *zap.Logger
}

// New creates a Logger from an zap.Logger.
func New(l *zap.Logger) *Zap {
	return &Zap{logger: l}
}

// WithFields creates a new logger with fields
func (l *Zap) WithFields(fields ...logger.Field) logger.Logger {
	var zapFields []zap.Field
	for _, field := range fields {
		zapFields = append(zapFields, zap.String(field.Key, field.Value))
	}
	return New(l.logger.With(zapFields...))
}

// Debug logs a debug message
func (l *Zap) Debug(msg string) {
	l.logger.Debug(msg)
}
