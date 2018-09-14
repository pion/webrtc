package dtls

/*
#cgo linux windows pkg-config: libssl libcrypto
#cgo linux CFLAGS: -Wno-deprecated-declarations
#cgo darwin CFLAGS: -I/usr/local/opt/openssl/include -I/usr/local/opt/openssl/include -Wno-deprecated-declarations
#cgo darwin LDFLAGS: -L/usr/local/opt/openssl/lib -L/usr/local/opt/openssl/lib -lssl -lcrypto
#cgo windows CFLAGS: -DWIN32_LEAN_AND_MEAN

#include "queue.h"
#include "dtls.h"

*/
import "C"
import (
	"container/list"
	"sync"
	"unsafe"

	"github.com/pkg/errors"
)

// export go_callback
func go_callback() {}

func init() {
	C.dtls_init()
}

// var listenerMap = make(map[string]*ipv4.PacketConn)
// var listenerMapLock = &sync.Mutex{}

// export go_handle_sendto
// func go_handle_sendto(rawBuf *C.char, rawBufLen C.int) {
// 	local := C.GoString(rawLocal)
// 	remote := C.GoString(rawRemote)
// 	buf := []byte(C.GoStringN(rawBuf, rawBufLen))
// 	C.free(unsafe.Pointer(rawBuf))
//
// 	listenerMapLock.Lock()
// 	defer listenerMapLock.Unlock()
// 	if conn, ok := listenerMap[local]; ok {
// 		strIP, strPort, err := net.SplitHostPort(remote)
// 		if err != nil {
// 			fmt.Println(err)
// 			return
// 		}
// 		port, err := strconv.Atoi(strPort)
// 		if err != nil {
// 			fmt.Println(err)
// 			return
// 		}
// 		_, err = conn.WriteTo(buf, nil, &net.UDPAddr{IP: net.ParseIP(strIP), Port: port})
// 		if err != nil {
// 			fmt.Println(err)
// 		}
// 	} else {
// 		fmt.Printf("Could not find ipv4.PacketConn for %s \n", local)
// 	}
// }

// State represents all the state needed for a DTLS session
type State struct {
	sync.Mutex

	queue *C.struct_queue_st
	cert  *C.struct_dtls_cert_st
	ctx   *C.struct_ssl_ctx_st
	sess  *C.struct_dtls_sess_st

	Input  chan []byte
	Output chan []byte

	OnReceive func(ReceiveEvent)
}

// New creates a new DTLS session
func NewState() *State {
	state := &State{
		queue: C.queue_init(),
		cert:  C.dtls_build_certificate(),

		Input:  make(chan []byte, 1),
		Output: make(chan []byte, 1),
	}
	state.ctx = C.dtls_build_ssl_context(state.cert)

	go state.inboundHander()
	go state.outboundHandler()
	return state
}

func (s *State) inboundHander() {
	reader := make(chan []byte, 1)
	queue := list.New()
	for {
		if front := queue.Front(); front == nil {
			if s.Input == nil {
				return
			}

			value, ok := <-s.Input
			if !ok {
				return
			}
			queue.PushBack(value)
		} else {
			select {
			case reader <- front.Value.([]byte):
				buffer := <-reader

				if 127 < buffer[0] && buffer[0] < 192 {
					// p.handleSRTP(buffer)
				} else if 19 < buffer[0] && buffer[0] < 64 {
					// decrypted, err := s.handleInbound(buffer)
					// if err != nil {
					// 	fmt.Println(err)
					// 	return
					// }
					//
					// if len(decrypted) > 0 {
					// 	if s.OnReceive != nil {
					// 		go s.OnReceive(ReceiveEvent{
					// 			Buffer: decrypted,
					// 		})
					// 	}
					// }

					// p.m.certPairLock.Lock()
					// if certPair := p.m.dtlsState.GetCertPair(); certPair != nil && p.m.certPair == nil {
					// 	p.m.certPair = certPair
					// }
					// p.m.certPairLock.Unlock()
				}

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

func (s *State) outboundHandler() {
	for {
		msg := C.queue_get(s.queue, nil)
		if msg == nil {
			break
		}

	}
}

// Start allocates DTLS/ICE state that is dependent on if we are offering or answering
func (s *State) Start(isOffer bool) {
	s.sess = C.dtls_build_session(s.ctx, C.bool(isOffer))
}

// Close cleans up the associated OpenSSL resources
func (s *State) Close() {
	C.queue_destroy(s.queue)
	C.dtls_session_cleanup(s.ctx, s.sess, s.cert)
}

// Fingerprint generates a SHA-256 fingerprint of the certificate
func (s *State) Fingerprint() string {
	rawFingerprint := C.dtls_certificate_fingerprint(s.cert)
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
func (s *State) handleInbound(packet []byte) ([]byte, error) {
	s.Lock()
	defer s.Unlock()

	if s.sess == nil {
		return nil, errors.Errorf("Unable to handle DTLS packet, session has not started")
	}

	// rawLocal := C.CString(local)
	// rawRemote := C.CString(remote)
	packetRaw := C.CBytes(packet) // unsafe.Pointer
	defer func() {
		// C.free(unsafe.Pointer(rawLocal))
		// C.free(unsafe.Pointer(rawRemote))
		C.free(packetRaw)
	}()

	if ret := C.dtls_handle_incoming(s.sess, s.queue, packetRaw, C.int(len(packet))); ret != nil {
		defer func() {
			C.free(ret.data)
			C.free(unsafe.Pointer(ret))
		}()
		return C.GoBytes(ret.data, ret.size), nil
	}
	return nil, nil
}

// Send takes a un-encrypted packet and sends via DTLS
func (s *State) Send(packet []byte) (bool, error) {
	s.Lock()
	defer s.Unlock()

	if s.sess == nil {
		return false, errors.Errorf("Unable to send via DTLS, session has not started")
	}

	// rawLocal := C.CString(local)
	// rawRemote := C.CString(remote)
	packetRaw := C.CBytes(packet) // unsafe.Pointer
	defer func() {
		// C.free(unsafe.Pointer(rawLocal))
		// C.free(unsafe.Pointer(rawRemote))
		C.free(packetRaw)
	}()

	return bool(C.dtls_handle_outgoing(s.sess, s.queue, packetRaw, C.int(len(packet)))), nil
}

// GetCertPair gets the current CertPair if DTLS has finished
func (s *State) GetCertPair() *CertPair {
	s.Lock()
	defer s.Unlock()

	if s.sess == nil {
		return nil
	}

	if ret := C.dtls_get_certpair(s.sess); ret != nil {
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
	if s.sess == nil {
		return
	}

	rawLocal := C.CString(local)
	rawRemote := C.CString(remote)
	defer func() {
		C.free(unsafe.Pointer(rawLocal))
		C.free(unsafe.Pointer(rawRemote))
	}()

	C.dtls_do_handshake(s.sess, s.queue, rawLocal, rawRemote)
}
