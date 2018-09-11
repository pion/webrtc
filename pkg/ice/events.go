package ice

type ReceiveEvent struct {
	Buffer []byte
	Local  string
	Remote string
}
