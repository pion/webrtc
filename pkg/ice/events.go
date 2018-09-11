package ice

// ReceiveEvent represents the data invoked on the ice.OnReceive method.
type ReceiveEvent struct {
	// Buffer represents the raw data received from the agent.
	Buffer []byte

	// Local represents the local transport address which handled the packet.
	Local string

	// Remote represents the remote transport address which sent the packet.
	Remote string
}
