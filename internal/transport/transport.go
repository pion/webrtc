package transport

// Conn is a minimal net.Conn
type Conn interface {
	Read([]byte) (n int, err error)
	Write([]byte) (n int, err error)
	Close() error
}
