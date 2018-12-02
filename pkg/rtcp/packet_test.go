package rtcp

import (
	"fmt"
)

func ExampleUnmarshal() {
	data := []byte{
		// RapidResynchronizationRequest
		0x85, 0xcd, 0x0, 0x2,
		// sender=0x902f9e2e
		0x90, 0x2f, 0x9e, 0x2e,
		// media=0x902f9e2e
		0x90, 0x2f, 0x9e, 0x2e,
	}
	packet, err := Unmarshal(data)
	if err != nil {
		panic(err)
	}

	switch p := packet.(type) {
	case *RapidResynchronizationRequest:
		fmt.Println(p.MediaSSRC)
	case *RawPacket:
		fmt.Printf("unknown packet type: %d\n", p.Header().Type)
	}
	// Output: 2419039790
}
