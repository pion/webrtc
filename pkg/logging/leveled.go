package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"sync/atomic"
)

// LogLevel represents the level at which the logger will emit log messages
type LogLevel int32

// Set updates the LogLevel to the supplied value
func (ll *LogLevel) Set(newLevel LogLevel) {
	atomic.StoreInt32((*int32)(ll), int32(newLevel))
}

// Get retrieves the current LogLevel value
func (ll *LogLevel) Get() LogLevel {
	return LogLevel(atomic.LoadInt32((*int32)(ll)))
}

func (ll LogLevel) String() string {
	switch ll {
	case LogLevelDisabled:
		return "Disabled"
	case LogLevelError:
		return "Error"
	case LogLevelWarn:
		return "Warn"
	case LogLevelInfo:
		return "Info"
	case LogLevelDebug:
		return "Debug"
	case LogLevelTrace:
		return "Trace"
	default:
		return "UNKNOWN"
	}
}

const (
	// LogLevelDisabled completely disables logging of any events
	LogLevelDisabled LogLevel = iota
	// LogLevelError is for fatal errors which should be handled by user code,
	// but are logged to ensure that they are seen
	LogLevelError
	// LogLevelWarn is for logging abnormal, but non-fatal library operation
	LogLevelWarn
	// LogLevelInfo is for logging normal library operation (e.g. state transitions, etc.)
	LogLevelInfo
	// LogLevelDebug is for logging low-level library information (e.g. internal operations)
	LogLevelDebug
	// LogLevelTrace is for logging very low-level library information (e.g. network traces)
	LogLevelTrace
)

// Use this abstraction to ensure thread-safe access to the logger's io.Writer
// (which could change at runtime)
type loggerWriter struct {
	sync.RWMutex
	output io.Writer
}

func (lw *loggerWriter) SetOutput(output io.Writer) {
	lw.Lock()
	defer lw.Unlock()
	lw.output = output
}

func (lw *loggerWriter) Write(data []byte) (int, error) {
	lw.RLock()
	defer lw.RUnlock()
	return lw.output.Write(data)
}

// provide a package-level default destination that can be changed
// at runtime
var defaultWriter = &loggerWriter{
	output: os.Stdout,
}

// SetDefaultWriter changes the default logging destination to the
// supplied io.Writer
func SetDefaultWriter(w io.Writer) {
	defaultWriter.SetOutput(w)
}

// LeveledLogger encapsulates functionality for providing logging at
// user-defined levels
type LeveledLogger struct {
	level  LogLevel
	writer *loggerWriter
	trace  *log.Logger
	debug  *log.Logger
	info   *log.Logger
	warn   *log.Logger
	err    *log.Logger
}

// WithTraceLogger is a chainable configuration function which sets the
// Trace-level logger
func (ll *LeveledLogger) WithTraceLogger(log *log.Logger) *LeveledLogger {
	ll.trace = log
	return ll
}

// WithDebugLogger is a chainable configuration function which sets the
// Debug-level logger
func (ll *LeveledLogger) WithDebugLogger(log *log.Logger) *LeveledLogger {
	ll.debug = log
	return ll
}

// WithInfoLogger is a chainable configuration function which sets the
// Info-level logger
func (ll *LeveledLogger) WithInfoLogger(log *log.Logger) *LeveledLogger {
	ll.info = log
	return ll
}

// WithWarnLogger is a chainable configuration function which sets the
// Warn-level logger
func (ll *LeveledLogger) WithWarnLogger(log *log.Logger) *LeveledLogger {
	ll.warn = log
	return ll
}

// WithErrorLogger is a chainable configuration function which sets the
// Error-level logger
func (ll *LeveledLogger) WithErrorLogger(log *log.Logger) *LeveledLogger {
	ll.err = log
	return ll
}

// WithLogLevel is a chainable configuration function which sets the logger's
// logging level threshold, at or below which all messages will be logged
func (ll *LeveledLogger) WithLogLevel(level LogLevel) *LeveledLogger {
	ll.level.Set(level)
	return ll
}

// WithOutput is a chainable configuration function which sets the logger's
// logging output to the supplied io.Writer
func (ll *LeveledLogger) WithOutput(output io.Writer) *LeveledLogger {
	ll.writer.SetOutput(output)
	return ll
}

// SetLevel sets the logger's logging level
func (ll *LeveledLogger) SetLevel(newLevel LogLevel) {
	ll.level.Set(newLevel)
}

func (ll *LeveledLogger) logf(logger *log.Logger, level LogLevel, format string, args ...interface{}) {
	if ll.level.Get() < level {
		return
	}

	callDepth := 3 // this frame + wrapper func + caller
	msg := fmt.Sprintf(format, args...)
	if err := logger.Output(callDepth, msg); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to log: %s", err)
	}
}

// Trace emits the preformatted message if the logger is at or below LogLevelTrace
func (ll *LeveledLogger) Trace(msg string) {
	ll.logf(ll.trace, LogLevelTrace, msg)
}

// Tracef formats and emits a message if the logger is at or below LogLevelTrace
func (ll *LeveledLogger) Tracef(format string, args ...interface{}) {
	ll.logf(ll.trace, LogLevelTrace, format, args...)
}

// Debug emits the preformatted message if the logger is at or below LogLevelDebug
func (ll *LeveledLogger) Debug(msg string) {
	ll.logf(ll.debug, LogLevelDebug, msg)
}

// Debugf formats and emits a message if the logger is at or below LogLevelDebug
func (ll *LeveledLogger) Debugf(format string, args ...interface{}) {
	ll.logf(ll.debug, LogLevelDebug, format, args...)
}

// Info emits the preformatted message if the logger is at or below LogLevelInfo
func (ll *LeveledLogger) Info(msg string) {
	ll.logf(ll.info, LogLevelInfo, msg)
}

// Infof formats and emits a message if the logger is at or below LogLevelInfo
func (ll *LeveledLogger) Infof(format string, args ...interface{}) {
	ll.logf(ll.info, LogLevelInfo, format, args...)
}

// Warn emits the preformatted message if the logger is at or below LogLevelWarn
func (ll *LeveledLogger) Warn(msg string) {
	ll.logf(ll.warn, LogLevelWarn, msg)
}

// Warnf formats and emits a message if the logger is at or below LogLevelWarn
func (ll *LeveledLogger) Warnf(format string, args ...interface{}) {
	ll.logf(ll.warn, LogLevelWarn, format, args...)
}

// Error emits the preformatted message if the logger is at or below LogLevelError
func (ll *LeveledLogger) Error(msg string) {
	ll.logf(ll.err, LogLevelError, msg)
}

// Errorf formats and emits a message if the logger is at or below LogLevelError
func (ll *LeveledLogger) Errorf(format string, args ...interface{}) {
	ll.logf(ll.err, LogLevelError, format, args...)
}

// NewLeveledLogger returns a configured *LeveledLogger
func NewLeveledLogger() *LeveledLogger {
	return NewLeveledLoggerForScope("PIONS")
}

// NewLeveledLoggerForScope returns a configured *LeveledLogger for the given scope
func NewLeveledLoggerForScope(scope string) *LeveledLogger {
	logger := &LeveledLogger{
		writer: &loggerWriter{
			output: defaultWriter,
		},
		level: LogLevelError, // TODO: Should this be the default? Or disabled?
	}
	return logger.
		WithTraceLogger(log.New(logger.writer, fmt.Sprintf("%s TRACE: ", scope), log.Lmicroseconds|log.Lshortfile)).
		WithDebugLogger(log.New(logger.writer, fmt.Sprintf("%s DEBUG: ", scope), log.Lmicroseconds|log.Lshortfile)).
		WithInfoLogger(log.New(logger.writer, fmt.Sprintf("%s INFO: ", scope), log.LstdFlags)).
		WithWarnLogger(log.New(logger.writer, fmt.Sprintf("%s WARNING: ", scope), log.LstdFlags)).
		WithErrorLogger(log.New(logger.writer, fmt.Sprintf("%s ERROR: ", scope), log.LstdFlags))
}
