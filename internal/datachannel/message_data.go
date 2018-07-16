package datachannel

import (
	"github.com/pkg/errors"
)

/*
Data isn't really a message type, just the default if we can't parse

I haven't found an RFC link yet around this
*/
type Data struct {
	Data []byte
}

// Marshal returns raw bytes for the given message
func (d *Data) Marshal() ([]byte, error) {
	return nil, errors.Errorf("Unimplemented")
}

// Unmarshal populates the struct with the given raw data
func (d *Data) Unmarshal(raw []byte) error {
	d.Data = raw
	return nil
}
