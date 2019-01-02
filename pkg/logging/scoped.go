package logging

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

type loggerRegistry struct {
	sync.RWMutex
	scopeLoggers map[string]*LeveledLogger
	scopeLevels  map[string]LogLevel
}

var (
	registry = &loggerRegistry{
		scopeLoggers: make(map[string]*LeveledLogger),
		scopeLevels:  make(map[string]LogLevel),
	}
)

// SetLogLevelForScope sets the logging level for the given
// scope, or all scopes if "all" is provided. If a logger
// for the scope does not yet exist, the desired logging
// level is recorded and applied when the scoped logger
// is created.
func SetLogLevelForScope(scope string, level LogLevel) {
	registry.Lock()
	defer registry.Unlock()

	scope = strings.ToLower(scope)
	registry.scopeLevels[scope] = level

	if scope == "all" {
		for _, logger := range registry.scopeLoggers {
			logger.SetLevel(level)
		}
		return
	}

	if logger, found := registry.scopeLoggers[scope]; found {
		logger.SetLevel(level)
	}
}

// NewScopedLogger returns a predefined logger for the given logging scope
// NB: Can be used idempotently
func NewScopedLogger(scope string) *LeveledLogger {
	registry.Lock()
	defer registry.Unlock()

	scope = strings.ToLower(scope)
	if _, found := registry.scopeLoggers[scope]; !found {
		registry.scopeLoggers[scope] = NewLeveledLoggerForScope(scope)

		// Handle a logger being created after init() is run
		level := LogLevelDisabled
		if allLevel, found := registry.scopeLevels["all"]; found {
			level = allLevel
		}
		if scopeLevel, found := registry.scopeLevels[scope]; found {
			if scopeLevel > level {
				level = scopeLevel
			}
		}
		if level > LogLevelDisabled {
			registry.scopeLoggers[scope].SetLevel(level)
		}
	}
	return registry.scopeLoggers[scope]
}

func init() {
	logLevels := map[string]LogLevel{
		"ERROR": LogLevelError,
		"WARN":  LogLevelWarn,
		"INFO":  LogLevelInfo,
		"DEBUG": LogLevelDebug,
		"TRACE": LogLevelTrace,
	}

	for name, level := range logLevels {
		env := os.Getenv(fmt.Sprintf("PIONS_LOG_%s", name))
		if env == "" {
			continue
		}

		if strings.ToLower(env) == "all" {
			for _, logger := range registry.scopeLoggers {
				logger.SetLevel(level)
			}
			registry.scopeLevels["all"] = level
			continue
		}

		scopes := strings.Split(strings.ToLower(env), ",")
		for _, scope := range scopes {
			registry.scopeLevels[scope] = level
			if logger, found := registry.scopeLoggers[strings.TrimSpace(scope)]; found {
				logger.SetLevel(level)
			}
		}
	}
}
