package logging_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/pions/webrtc/pkg/logging"
)

func TestScopedLogger(t *testing.T) {
	var outBuf bytes.Buffer
	logger := logging.NewScopedLogger("test1").
		WithOutput(&outBuf).
		WithLogLevel(logging.LogLevelWarn)

	logger.Debug("this shouldn't be logged")
	if outBuf.Len() > 0 {
		t.Error("Debug was logged when it shouldn't have been")
	}
	logger.Debugf("this shouldn't be logged")
	if outBuf.Len() > 0 {
		t.Error("Debug was logged when it shouldn't have been")
	}

	warnMsg := "this is a warning message"
	logger.Warn(warnMsg)
	if !strings.Contains(outBuf.String(), warnMsg) {
		t.Errorf("Expected to find %q in %q, but didn't", warnMsg, outBuf.String())
	}
	logger.Warnf(warnMsg)
	if !strings.Contains(outBuf.String(), warnMsg) {
		t.Errorf("Expected to find %q in %q, but didn't", warnMsg, outBuf.String())
	}

	errMsg := "this is an error message"
	logger.Error(errMsg)
	if !strings.Contains(outBuf.String(), errMsg) {
		t.Errorf("Expected to find %q in %q, but didn't", errMsg, outBuf.String())
	}
	logger.Errorf(errMsg)
	if !strings.Contains(outBuf.String(), errMsg) {
		t.Errorf("Expected to find %q in %q, but didn't", errMsg, outBuf.String())
	}
}

func TestPackageLevelSettings(t *testing.T) {
	var outBuf bytes.Buffer
	logger := logging.NewScopedLogger("test2")

	// set the package-level writer
	logging.SetDefaultWriter(&outBuf)

	traceMsg := "this is a trace messages"
	logger.Trace(traceMsg)

	if outBuf.Len() > 0 {
		t.Error("Trace was logged when it shouldn't have been")
	}

	logger.Tracef(traceMsg)

	if outBuf.Len() > 0 {
		t.Error("Trace was logged when it shouldn't have been")
	}

	// set the logging scope via package
	logging.SetLogLevelForScope("test2", logging.LogLevelTrace)

	logger.Trace(traceMsg)
	if !strings.Contains(outBuf.String(), traceMsg) {
		t.Errorf("Expected to find %q in %q, but didn't", traceMsg, outBuf.String())
	}

	logger.Tracef(traceMsg)
	if !strings.Contains(outBuf.String(), traceMsg) {
		t.Errorf("Expected to find %q in %q, but didn't", traceMsg, outBuf.String())
	}
}
