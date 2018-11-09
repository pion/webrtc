package logging

import (
	"io/ioutil"
	"log"
	"os"
)

type logLevel int

const(
	Error = iota + 1
	Warn
	Info
	Debug
	Trace
)

// RTCPeerLogger is the default logging struct for rtcpeerconnection.
type RTCPeerLogger struct{
	info, debug, warning,
	trace, error  *log.Logger
}

// webrtcLogger is the interface that must be implemented if a custom
// logger is used.
type webrtcLogger interface{
	Info(msg string)
	Trace(msg string)
	Debug(msg string)
	Warning(msg string)
	Error(msg string)
}

func (pclog *RTCPeerLogger) Debug(debug string){
	pclog.debug.Println(debug)
}

func (pclog *RTCPeerLogger) Error(error string){
	pclog.error.Println(error)
}

func (pclog *RTCPeerLogger) Info(info string){
	pclog.info.Println(info)
}

func (pclog *RTCPeerLogger) Warning(warning string){
	pclog.warning.Println(warning)
}

func (pclog *RTCPeerLogger) Trace(trace string){
	pclog.trace.Println(trace)
}

func (pclog *RTCPeerLogger) SetLogLevel(l logLevel) {

	debugOut := ioutil.Discard
	infoOut := ioutil.Discard
	traceOut := ioutil.Discard
	warningOut := ioutil.Discard
	errorOut := ioutil.Discard

	switch l {
	case Error:
		errorOut = os.Stdout
	case Warn:
		errorOut = os.Stdout
		warningOut = os.Stdout
	case Info:
		errorOut = os.Stdout
		warningOut = os.Stdout
		infoOut = os.Stdout
	case Debug:
		errorOut = os.Stdout
		warningOut = os.Stdout
		infoOut = os.Stdout
		debugOut = os.Stdout
	case Trace:
		errorOut = os.Stdout
		warningOut = os.Stdout
		infoOut = os.Stdout
		debugOut = os.Stdout
		traceOut = os.Stdout
	}

	pclog.debug   = log.New(debugOut, "DEBUG: ", log.Lshortfile)
	pclog.info    = log.New(infoOut,  "INFO: ", log.Lshortfile)
	pclog.trace   = log.New(traceOut, "TRACE: ", log.Lshortfile)
	pclog.warning = log.New(warningOut,"WARNING: ", log.Lshortfile)
	pclog.error   = log.New(errorOut, "ERROR: ", log.Lshortfile)
}