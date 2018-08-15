package std

import (
	"fmt"
	"log"
	"strings"

	"github.com/pions/webrtc/pkg/logger"
)

type Level int

const (
	Debug = iota + 1
	Info
)

func (l Level) String() string {
	switch l {
	case Debug:
		return "DEBUG"
	case Info:
		return "INFO"
	default:
		return "UNKNOWN"
	}
}

// Log is a Logger based on the golang log
type Log struct {
	logger *log.Logger
	parent *Log
	fields map[string]string
	level  Level
}

// New creates a logger.Logger from an log.Logger.
func New(l *log.Logger, level Level) *Log {
	return &Log{
		logger: l,
		level:  level,
	}
}

// WithFields creates a new logger with fields
func (l *Log) WithFields(fields ...logger.Field) logger.Logger {
	f := make(map[string]string)
	for k, v := range l.fields {
		f[k] = v
	}
	for _, field := range fields {
		f[field.Key] = field.Value
	}

	return &Log{
		parent: l,
		level:  l.level,
		fields: f,
	}
}

// Debug logs a debug message
func (l *Log) Debug(msg string) {
	l.print(Debug, msg)
}

func (l *Log) print(level Level, msg string) {
	fields := ""
	if l.fields != nil &&
		len(l.fields) > 0 {
		var parts []string
		for k, v := range l.fields {
			parts = append(parts, fmt.Sprintf("%s=%s", k, v))
		}
		fields = strings.Join(parts, ";") + " "
	}
	if l.checkLevel(level) {
		l.doPrint(fmt.Sprintf("%s%s: %s", fields, level, msg))
	}
}

func (l *Log) doPrint(msg string) {
	if l.logger != nil {
		l.logger.Print(msg)
	} else {
		l.parent.doPrint(msg)
	}
}

func (l *Log) checkLevel(level Level) bool {
	return int(level) >= int(l.level)
}
