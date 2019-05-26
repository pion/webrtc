package ice

// ConnectionState indicates signaling state of the ICE Connection.
type ConnectionState int

const (
	// ConnectionStateNew indicates that any of the ICETransports are
	// in the "new" state and none of them are in the "checking", "disconnected"
	// or "failed" state, or all ICETransports are in the "closed" state, or
	// there are no transports.
	ConnectionStateNew ConnectionState = iota + 1

	// ConnectionStateChecking indicates that any of the ICETransports
	// are in the "checking" state and none of them are in the "disconnected"
	// or "failed" state.
	ConnectionStateChecking

	// ConnectionStateConnected indicates that all ICETransports are
	// in the "connected", "completed" or "closed" state and at least one of
	// them is in the "connected" state.
	ConnectionStateConnected

	// ConnectionStateCompleted indicates that all ICETransports are
	// in the "completed" or "closed" state and at least one of them is in the
	// "completed" state.
	ConnectionStateCompleted

	// ConnectionStateDisconnected indicates that any of the
	// ICETransports are in the "disconnected" state and none of them are
	// in the "failed" state.
	ConnectionStateDisconnected

	// ConnectionStateFailed indicates that any of the ICETransports
	// are in the "failed" state.
	ConnectionStateFailed

	// ConnectionStateClosed indicates that the PeerConnection's
	// isClosed is true.
	ConnectionStateClosed
)

// This is done this way because of a linter.
const (
	connectionStateNewStr          = "new"
	connectionStateCheckingStr     = "checking"
	connectionStateConnectedStr    = "connected"
	connectionStateCompletedStr    = "completed"
	connectionStateDisconnectedStr = "disconnected"
	connectionStateFailedStr       = "failed"
	connectionStateClosedStr       = "closed"
)

// NewConnectionState takes a string and converts it to ConnectionState
func NewConnectionState(raw string) ConnectionState {
	switch raw {
	case connectionStateNewStr:
		return ConnectionStateNew
	case connectionStateCheckingStr:
		return ConnectionStateChecking
	case connectionStateConnectedStr:
		return ConnectionStateConnected
	case connectionStateCompletedStr:
		return ConnectionStateCompleted
	case connectionStateDisconnectedStr:
		return ConnectionStateDisconnected
	case connectionStateFailedStr:
		return ConnectionStateFailed
	case connectionStateClosedStr:
		return ConnectionStateClosed
	default:
		return ConnectionState(Unknown)
	}
}

func (c ConnectionState) String() string {
	switch c {
	case ConnectionStateNew:
		return connectionStateNewStr
	case ConnectionStateChecking:
		return connectionStateCheckingStr
	case ConnectionStateConnected:
		return connectionStateConnectedStr
	case ConnectionStateCompleted:
		return connectionStateCompletedStr
	case ConnectionStateDisconnected:
		return connectionStateDisconnectedStr
	case ConnectionStateFailed:
		return connectionStateFailedStr
	case ConnectionStateClosed:
		return connectionStateClosedStr
	default:
		return ErrUnknownType.Error()
	}
}
