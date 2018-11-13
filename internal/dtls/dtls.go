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
	"sync"
	"unsafe"

	"github.com/pions/webrtc/internal/transport"
	"github.com/pkg/errors"
)

func init() {
	if !C.openssl_global_init() {
		panic("Failed to initalize OpenSSL") // nolint
	}
}

// TODO FIXME
var currentState *State

// ConnectionState determines the DTLS connection state
type ConnectionState uint8

// ConnectionState enums
const (
	New ConnectionState = iota + 1
	Established
)

func (a ConnectionState) String() string {
	switch a {
	case New:
		return "New"
	case Established:
		return "Established"
	default:
		return fmt.Sprintf("Invalid ConnectionState %d", a)
	}
}

//export go_handle_sendto
func go_handle_sendto(rawLocal *C.char, rawRemote *C.char, rawBuf *C.char, rawBufLen C.int) {
	local := C.GoString(rawLocal)
	_ = local
	remote := C.GoString(rawRemote)
	_ = remote
	buf := []byte(C.GoStringN(rawBuf, rawBufLen))
	C.free(unsafe.Pointer(rawBuf))

	_, err := currentState.conn.Write(buf)
	if err != nil {
		fmt.Println("Failed go_handle_sendto", err)
	}
}

// State represents all the state needed for a DTLS session
type State struct {
	sync.Mutex

	state    ConnectionState
	notifier func(ConnectionState)

	tlscfg      *_Ctype_struct_tlscfg
	sslctx      *_Ctype_struct_ssl_ctx_st
	dtlsSession *_Ctype_struct_dtls_sess

	conn transport.Conn
}

// NewState creates a new DTLS session
func NewState(notifier func(ConnectionState)) (s *State, err error) {
	s = &State{
		tlscfg:   C.dtls_build_tlscfg(),
		state:    New,
		notifier: notifier,
	}

	s.sslctx = C.dtls_build_sslctx(s.tlscfg)

	currentState = s

	return s, err
}

// Start allocates DTLS/ICE state that is dependent on if we are offering or answering
func (s *State) Start(isOffer bool, conn transport.Conn) {
	s.conn = conn
	s.dtlsSession = C.dtls_build_session(s.sslctx, C.bool(isOffer))
}

func (s *State) setState(state ConnectionState) {
	if s.state != state {
		s.state = state
		if s.notifier != nil {
			go s.notifier(state)
		}
	}
}

// Close cleans up the associated OpenSSL resources
func (s *State) Close() {
	C.dtls_session_cleanup(s.sslctx, s.dtlsSession, s.tlscfg)
}

// Fingerprint generates a SHA-256 fingerprint of the certificate
func (s *State) Fingerprint() string {
	cfg := s.tlscfg
	if cfg == nil {
		return ""
	}
	var size uint
	var fingerprint [C.EVP_MAX_MD_SIZE]byte
	sizePtr := unsafe.Pointer(&size)
	fingerprintPtr := unsafe.Pointer(&fingerprint)
	if C.X509_digest(cfg.cert, C.EVP_sha256(), (*C.uchar)(fingerprintPtr), (*C.uint)(sizePtr)) == 0 {
		return ""
	}
	var hexFingerprint string
	for i := uint(0); i < size; i++ {
		hexFingerprint += fmt.Sprintf("%.2X:", fingerprint[i])
	}
	hexFingerprint = hexFingerprint[:len(hexFingerprint)-1]
	return hexFingerprint
}

// CertPair is the client+server key and profile extracted for SRTP
type CertPair struct {
	ClientWriteKey []byte
	ServerWriteKey []byte
	Profile        string
}

// HandleDTLSPacket checks if the packet is a DTLS packet, and if it is passes to the DTLS session
// If there is any data after decoding we pass back to the caller to handler
func (s *State) HandleDTLSPacket(packet []byte, local, remote string) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	if s.dtlsSession == nil {
		return nil, errors.Errorf("Unable to handle DTLS packet, session has not started")
	}

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	packetRaw := C.CBytes(packet) // unsafe.Pointer
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
		C.free(packetRaw)
	}()

	if ret := C.dtls_handle_incoming(s.dtlsSession, packetRaw, C.int(len(packet)), rawLocal, rawRemote); ret != nil {
		defer func() {
			C.free(ret.buf)
			C.free(unsafe.Pointer(ret))
		}()

		if bool(ret.init) && s.state == New {
			s.setState(Established)
		}

		return C.GoBytes(ret.buf, ret.len), nil
	}
	return nil, nil
}

// Send takes a un-encrypted packet and sends via DTLS
func (s *State) Send(packet []byte, local, remote string) (bool, error) {
	s.Lock()
	defer s.Unlock()

	if s.dtlsSession == nil {
		return false, errors.Errorf("Unable to send via DTLS, session has not started")
	}

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	packetRaw := C.CBytes(packet) // unsafe.Pointer
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
		C.free(packetRaw)
	}()

	return bool(C.dtls_handle_outgoing(s.dtlsSession, packetRaw, C.int(len(packet)), rawLocal, rawRemote)), nil
}

// GetCertPair gets the current CertPair if DTLS has finished
func (s *State) GetCertPair() *CertPair {
	s.Lock()
	defer s.Unlock()

	if s.dtlsSession == nil {
		return nil
	}

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
	if s.dtlsSession == nil {
		return
	}

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
	}()

	C.dtls_do_handshake(s.dtlsSession, rawLocal, rawRemote)
}
