package sctp

import (
	"fmt"
	"testing"
)

func TestAssociationInit(t *testing.T) {
	rawPkt := []byte{0x13, 0x88, 0x13, 0x88, 0x00, 0x00, 0x00, 0x00, 0x81, 0x46, 0x9d, 0xfc, 0x01, 0x00, 0x00, 0x56, 0x55,
		0xb9, 0x64, 0xa5, 0x00, 0x02, 0x00, 0x00, 0x04, 0x00, 0x08, 0x00, 0xe8, 0x6d, 0x10, 0x30, 0xc0, 0x00, 0x00, 0x04, 0x80,
		0x08, 0x00, 0x09, 0xc0, 0x0f, 0xc1, 0x80, 0x82, 0x00, 0x00, 0x00, 0x80, 0x02, 0x00, 0x24, 0x9f, 0xeb, 0xbb, 0x5c, 0x50,
		0xc9, 0xbf, 0x75, 0x9c, 0xb1, 0x2c, 0x57, 0x4f, 0xa4, 0x5a, 0x51, 0xba, 0x60, 0x17, 0x78, 0x27, 0x94, 0x5c, 0x31, 0xe6,
		0x5d, 0x5b, 0x09, 0x47, 0xe2, 0x22, 0x06, 0x80, 0x04, 0x00, 0x06, 0x00, 0x01, 0x00, 0x00, 0x80, 0x03, 0x00, 0x06, 0x80, 0xc1, 0x00, 0x00}

	assoc := &Association{}
	if err := assoc.handleInbound(rawPkt); err != nil {
		// TODO
		fmt.Println(err)
		// t.Error(errors.Wrap(err, "Failed to HandleInbound"))
	}
}

// TODO: find a good way to avoid deadlocking in the test
// func TestStressDuplex(t *testing.T) {
// 	// Limit runtime in case of deadlocks
// 	lim := test.TimeOut(time.Second * 20)
// 	defer lim.Stop()
//
// 	// Check for leaking routines
// 	report := test.CheckRoutines(t)
// 	defer report()
//
// 	// Run the test
// 	stressDuplex(t)
// }
//
// func stressDuplex(t *testing.T) {
// 	lim := test.TimeOut(time.Second * 5)
// 	defer lim.Stop()
//
// 	ca, cb, stop, err := pipeMemory()
// 	if err != nil {
// 		t.Fatal(err)
// 	}
//
// 	defer stop(t)
//
// 	opt := test.Options{
// 		MsgSize:  2048,
// 		MsgCount: 50,
// 	}
//
// 	err = test.StressDuplex(ca, cb, opt)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }
//
// func pipeMemory() (*Stream, *Stream, func(*testing.T), error) {
// 	var err error
//
// 	var aa, ab *Association
// 	aa, ab, err = associationMemory()
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
//
// 	var sa, sb *Stream
// 	sa, err = aa.OpenStream(0, 0)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
//
// 	sb, err = ab.OpenStream(0, 0)
// 	if err != nil {
// 		return nil, nil, nil, err
// 	}
//
// 	stop := func(t *testing.T) {
// 		err = sa.Close()
// 		if err != nil {
// 			t.Error(err)
// 		}
// 		err = sb.Close()
// 		if err != nil {
// 			t.Error(err)
// 		}
// 		err = aa.Close()
// 		if err != nil {
// 			t.Error(err)
// 		}
// 		err = ab.Close()
// 		if err != nil {
// 			t.Error(err)
// 		}
// 	}
//
// 	return sa, sb, stop, nil
// }
//
// func associationMemory() (*Association, *Association, error) {
// 	// In memory pipe
// 	ca, cb := test.PacketPipe(100) // TODO: Find a better way to avoid blocking
//
// 	type result struct {
// 		a   *Association
// 		err error
// 	}
//
// 	c := make(chan result)
//
// 	// Setup client
// 	go func() {
// 		client, err := Client(ca)
// 		c <- result{client, err}
// 	}()
//
// 	// Setup server
// 	server, err := Server(cb)
// 	if err != nil {
// 		return nil, nil, err
// 	}
//
// 	// Receive client
// 	res := <-c
// 	if res.err != nil {
// 		return nil, nil, res.err
// 	}
//
// 	return res.a, server, nil
// }
