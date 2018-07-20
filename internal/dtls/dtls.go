package dtls

/*
#cgo linux windows pkg-config: libssl libcrypto
#cgo linux CFLAGS: -Wno-deprecated-declarations
#cgo darwin CFLAGS: -I/usr/local/opt/openssl/include -I/usr/local/opt/openssl/include -Wno-deprecated-declarations
#cgo darwin LDFLAGS: -L/usr/local/opt/openssl/lib -L/usr/local/opt/openssl/lib -lssl -lcrypto
#cgo windows CFLAGS: -DWIN32_LEAN_AND_MEAN

#include "dtls.h"

*/
import "C"
import (
	"fmt"
	"net"
	"strconv"
	"sync"
	"unsafe"

	"golang.org/x/net/ipv4"
)

func init() {
	if !C.openssl_global_init() {
		panic("Failed to initalize OpenSSL")
	}
}

var listenerMap = make(map[string]*ipv4.PacketConn)
var listenerMapLock = &sync.Mutex{}

//export go_handle_sendto
func go_handle_sendto(rawLocal *C.char, rawRemote *C.char, rawBuf *C.char, rawBufLen C.int) {
	local := C.GoString(rawLocal)
	remote := C.GoString(rawRemote)
	buf := []byte(C.GoStringN(rawBuf, rawBufLen))
	C.free(unsafe.Pointer(rawBuf))

	listenerMapLock.Lock()
	defer listenerMapLock.Unlock()
	if conn, ok := listenerMap[local]; ok {
		strIP, strPort, err := net.SplitHostPort(remote)
		if err != nil {
			fmt.Println(err)
			return
		}
		port, err := strconv.Atoi(strPort)
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = conn.WriteTo(buf, nil, &net.UDPAddr{IP: net.ParseIP(strIP), Port: port})
		if err != nil {
			fmt.Println(err)
		}
	} else {
		fmt.Printf("Could not find ipv4.PacketConn for %s \n", local)
	}
}

// State represents all the state needed for a DTLS session
type State struct {
	sync.Mutex

	tlscfg      *_Ctype_struct_tlscfg
	sslctx      *_Ctype_struct_ssl_ctx_st
	dtlsSession *_Ctype_struct_dtls_sess
}

// NewState creates a new DTLS session
func NewState(isClient bool) (s *State, err error) {
	s = &State{
		tlscfg: C.dtls_build_tlscfg(),
	}

	s.sslctx = C.dtls_build_sslctx(s.tlscfg)
	s.dtlsSession = C.dtls_build_session(s.sslctx, C.bool(!isClient))

	return s, err
}

// Close cleans up the associated OpenSSL resources
func (s *State) Close() {
	C.dtls_session_cleanup(s.sslctx, s.dtlsSession, s.tlscfg)
}

// Fingerprint generates a SHA-256 fingerprint of the certificate
func (s *State) Fingerprint() string {
	rawFingerprint := C.dtls_tlscfg_fingerprint(s.tlscfg)
	defer C.free(unsafe.Pointer(rawFingerprint))
	return C.GoString(rawFingerprint)
}

// CertPair is the client+server key and profile extracted for SRTP
type CertPair struct {
	ClientWriteKey []byte
	ServerWriteKey []byte
	Profile        string
}

// HandleDTLSPacket checks if the packet is a DTLS packet, and if it is passes to the DTLS session
// If there is any data after decoding we pass back to the caller to handler
func (s *State) HandleDTLSPacket(packet []byte, local, remote string) []byte {
	s.Lock()
	defer s.Unlock()

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	packetRaw := C.CBytes(packet)
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
		C.free(unsafe.Pointer(packetRaw))
	}()

	if ret := C.dtls_handle_incoming(s.dtlsSession, packetRaw, C.int(len(packet)), rawLocal, rawRemote); ret != nil {
		defer func() {
			C.free(unsafe.Pointer(ret.buf))
			C.free(unsafe.Pointer(ret))
		}()
		return []byte(C.GoBytes(ret.buf, ret.len))
	}
	return nil
}

// Send takes a un-encrypted packet and sends via DTLS
func (s *State) Send(packet []byte, local, remote string) bool {
	s.Lock()
	defer s.Unlock()

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	packetRaw := C.CBytes(packet)
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
		C.free(unsafe.Pointer(packetRaw))
	}()

	return bool(C.dtls_handle_outgoing(s.dtlsSession, packetRaw, C.int(len(packet)), rawLocal, rawRemote))
}

// GetCertPair gets the current CertPair if DTLS has finished
func (s *State) GetCertPair() *CertPair {
	if ret := C.dtls_get_certpair(s.dtlsSession); ret != nil {
		defer C.free(unsafe.Pointer(ret))
		return &CertPair{
			ClientWriteKey: []byte(C.GoStringN(&ret.client_write_key[0], ret.key_length)),
			ServerWriteKey: []byte(C.GoStringN(&ret.server_write_key[0], ret.key_length)),
			Profile:        C.GoString(&ret.profile[0]),
		}
	}
	return nil
}

// DoHandshake sends the DTLS handshake it the remote peer
func (s *State) DoHandshake(local, remote string) {
	s.Lock()
	defer s.Unlock()

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
	}()

	C.dtls_do_handshake(s.dtlsSession, rawLocal, rawRemote)
}

// AddListener adds the socket to a map that can be accessed by OpenSSL for sending
// This only needed until DTLS is rewritten in native Go
func AddListener(src string, conn *ipv4.PacketConn) {
	listenerMapLock.Lock()
	listenerMap[src] = conn
	listenerMapLock.Unlock()
}

// RemoveListener removes the socket from a map that can be accessed by OpenSSL for sending
// This only needed until DTLS is rewritten in native Go
func RemoveListener(src string) {
	listenerMapLock.Lock()
	delete(listenerMap, src)
	listenerMapLock.Unlock()
}
