package srtp

/*
#cgo pkg-config: libsrtp2

#include "srtp.h"

*/
import "C"
import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"unsafe"

	"github.com/pions/webrtc/pkg/rtp"
	"github.com/pkg/errors"
)

func init() {
	C.srtp_init()
}

// Session containts the libsrtp state for this SRTP session
type Session struct {
	rawSession *_Ctype_srtp_t
	serverGcm  cipher.AEAD
}

// New creates a new SRTP Session
func New(ClientWriteKey, ServerWriteKey []byte, profile string) (*Session, error) {
	s := &Session{}

	block, err := aes.NewCipher(ServerWriteKey[0:16])
	if err != nil {
		return nil, err
	}

	s.serverGcm, err = cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	rawClientWriteKey := C.CBytes(ClientWriteKey)
	rawServerWriteKey := C.CBytes(ServerWriteKey)
	rawProfile := C.CString(profile)
	defer func() {
		C.free(unsafe.Pointer(rawClientWriteKey))
		C.free(unsafe.Pointer(rawServerWriteKey))
		C.free(unsafe.Pointer(rawProfile))
	}()

	if sess := C.srtp_create_session(rawClientWriteKey, rawServerWriteKey, rawProfile); sess != nil {
		s.rawSession = sess
		return s, nil
	}

	return nil, errors.Errorf("Failed to create libsrtp session")
}

// DecryptPacket decrypts a SRTP packet
func (s *Session) DecryptPacket(packet *rtp.Packet, rawEncryptedPacket []byte) bool {
	rawIn := C.CBytes(rawEncryptedPacket)
	defer C.free(unsafe.Pointer(rawIn))

	if rawPacket := C.srtp_decrypt_packet(s.rawSession, rawIn, C.int(len(rawEncryptedPacket))); rawPacket != nil {
		tmpPacket := &rtp.Packet{}
		if err := packet.Unmarshal(C.GoBytes(rawPacket.data, rawPacket.len)); err != nil {
			fmt.Println(err)
			return false
		}

		packet.Payload = tmpPacket.Payload
		return true
	}

	return false
}
