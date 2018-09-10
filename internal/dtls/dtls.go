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
	"container/list"
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

	Input  chan interface{}
	reader chan interface{}

	Output chan interface{}
	writer chan interface{}
}

// New creates a new DTLS session
func NewState() *State {
	state := &State{
		tlscfg: C.dtls_build_tlscfg(),
	}

	state.sslctx = C.dtls_build_sslctx(state.tlscfg)

	go state.inboundHander()
	go state.outboundHandler()
	return state
}

func (s *State) outboundHandler() {
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if s.writer == nil {
				close(s.Output)
				return
			}
			value, ok := <-s.writer
			if !ok {
				close(s.Output)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case s.Output <- front.Value:
				queue.Remove(front)
			case value, ok := <-s.writer:
				if ok {
					queue.PushBack(value)
				} else {
					s.writer = nil
				}
			}
		}
	}
}

func (s *State) inboundHander() {
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if s.Input == nil {
				close(s.reader)
				return
			}

			value, ok := <-s.Input
			if !ok {
				close(s.reader)
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case s.reader <- front.Value:
				raw := (<-s.reader).([]byte)
				queue.Remove(front)
			case value, ok := <-s.Input:
				if ok {
					queue.PushBack(value)
				} else {
					s.Input = nil
				}
			}
		}
	}
}

// Start allocates DTLS/ICE state that is dependent on if we are offering or answering
func (s *State) Start(isOffer bool) {
	s.dtlsSession = C.dtls_build_session(s.sslctx, C.bool(isOffer))
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
