package ice

// ConnectionState is an enum showing the state of a ICE Connection
type ConnectionState int

// List of supported States
const (
	// ConnectionStateNew ICE agent is gathering addresses
	ConnectionStateNew = iota + 1

	// ConnectionStateChecking ICE agent has been given local and remote candidates, and is attempting to find a match
	ConnectionStateChecking

	// ConnectionStateConnected ICE agent has a pairing, but is still checking other pairs
	ConnectionStateConnected

	// ConnectionStateCompleted ICE agent has finished
	ConnectionStateCompleted

	// ConnectionStateFailed ICE agent never could successfully connect
	ConnectionStateFailed

	// ConnectionStateDisconnected ICE agent connected successfully, but has entered a failed state
	ConnectionStateDisconnected

	// ConnectionStateClosed ICE agent has finished and is no longer handling requests
	ConnectionStateClosed
)

func (c ConnectionState) String() string {
	switch c {
	case ConnectionStateNew:
		return "New"
	case ConnectionStateChecking:
		return "Checking"
	case ConnectionStateConnected:
		return "Connected"
	case ConnectionStateCompleted:
		return "Completed"
	case ConnectionStateFailed:
		return "Failed"
	case ConnectionStateDisconnected:
		return "Disconnected"
	case ConnectionStateClosed:
		return "Closed"
	default:
		return "Invalid"
	}
}
