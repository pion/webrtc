package log

// Nil is a logger that drops all logs
type Nil struct{}

func (l *Nil) WithFields(...Field) Logger {
	return &Nil{}
}

func (l *Nil) Debug(msg string) {}
