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

	"github.com/pkg/errors"
	"golang.org/x/net/ipv4"
)

func init() {
	if !C.openssl_global_init() {
		panic("Failed to initalize OpenSSL") // nolint
	}
}

type DTLSState uint8

// DTLSState enums
const (
	New DTLSState = iota + 1
	Established
)

func (a DTLSState) String() string {
	switch a {
	case New:
		return "New"
	case Established:
		return "Established"
	default:
		return fmt.Sprintf("Invalid DTLSState %d", a)
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

	fmt.Println("go_handle_sendto", buf)
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

	started bool
	isOffer bool

	state    DTLSState
	notifier func(DTLSState)

	tlscfg      *_Ctype_struct_tlscfg
	sslctx      *_Ctype_struct_ssl_ctx_st
	dtlsSession *_Ctype_struct_dtls_sess
}

// NewState creates a new DTLS session
func NewState(notifier func(DTLSState)) (s *State, err error) {
	s = &State{
		tlscfg:   C.dtls_build_tlscfg(),
		state:    New,
		notifier: notifier,
	}

	s.sslctx = C.dtls_build_sslctx(s.tlscfg)

	return s, err
}

// Start allocates DTLS/ICE state that is dependent on if we are offering or answering
func (s *State) Start(isOffer bool) {
	s.started = true
	s.isOffer = isOffer
	s.dtlsSession = C.dtls_build_session(s.sslctx, C.bool(isOffer))
}

func (s *State) setState(state DTLSState) {
	if s.state != state {
		s.state = state
		go s.notifier(state)
	}
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
func (s *State) HandleDTLSPacket(packet []byte, local, remote string) ([]byte, error) {
	fmt.Println("HandleDTLSPacket", len(packet))
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
		fmt.Println("dtls_handle_incoming", ret.len)

		if bool(ret.init) && s.state == New {
			s.setState(Established)
		}

		return C.GoBytes(ret.buf, ret.len), nil
	}
	fmt.Println("HandleDTLSPacket nil")
	return nil, nil
}

// Send takes a un-encrypted packet and sends via DTLS
func (s *State) Send(packet []byte, local, remote string) (bool, error) {
	fmt.Println("DTLS send", packet)
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
	fmt.Println("DoHandshake", s.started, s.isOffer)
	defer s.Unlock()
	if s.dtlsSession == nil {
		fmt.Println("DoHandshake no session", s.started, s.isOffer)
		return
	}

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
	}()

	ret := C.dtls_do_handshake(s.dtlsSession, rawLocal, rawRemote)
	if ret < 0 {
		fmt.Println("DoHandshake failed", s.started, s.isOffer, ret)
	} else {
		fmt.Println("DoHandshake", s.started, s.isOffer, ret)
	}
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
