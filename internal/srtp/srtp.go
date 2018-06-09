package srtp

/*
#cgo pkg-config: libsrtp2

#include "srtp.h"

*/
import "C"
import (
	"fmt"
	"unsafe"
)

func init() {
	C.srtp_init()
}

type Session struct {
	rawSession *_Ctype_srtp_t
}

func New(ClientWriteKey, ServerWriteKey []byte, profile string) *Session {
	rawClientWriteKey := C.CBytes(ClientWriteKey)
	rawServerWriteKey := C.CBytes(ServerWriteKey)
	rawProfile := C.CString(profile)
	defer func() {
		C.free(unsafe.Pointer(rawClientWriteKey))
		C.free(unsafe.Pointer(rawServerWriteKey))
		C.free(unsafe.Pointer(rawProfile))
	}()

	if sess := C.srtp_create_session(rawClientWriteKey, rawServerWriteKey, rawProfile); sess != nil {
		return &Session{
			rawSession: sess,
		}
	}

	return nil
}

func (s *Session) DecryptPacket(encryted []byte) (ok bool, unencryted []byte) {
	rawIn := C.CBytes(encryted)
	defer C.free(unsafe.Pointer(rawIn))

	if rawPacket := C.srtp_decrypt_packet(s.rawSession, rawIn, C.int(len(encryted))); rawPacket != nil {
		fmt.Println(rawPacket)
		return true, C.GoBytes(rawPacket.data, rawPacket.len)
	}

	return ok, unencryted
}
