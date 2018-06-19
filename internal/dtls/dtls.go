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
		panic("Failed to initalize OpenSSL")
	}
}

var listenerMap = make(map[string]*ipv4.PacketConn)
var listenerMapLock = &sync.Mutex{}

//export go_handle_sendto
func go_handle_sendto(rawSrc *C.char, rawDst *C.char, rawBuf *C.char, rawBufLen C.int) {
	src := C.GoString(rawSrc)
	dst := C.GoString(rawDst)
	buf := []byte(C.GoStringN(rawBuf, rawBufLen))
	C.free(unsafe.Pointer(rawBuf))

	listenerMapLock.Lock()
	defer listenerMapLock.Unlock()
	if conn, ok := listenerMap[src]; ok {
		strIP, strPort, err := net.SplitHostPort(dst)
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
		fmt.Printf("Could not find ipv4.PacketConn for %s \n", src)
	}
}

// TLSCfg holds the Certificate/PrivateKey used for a single RTCPeerConnection
type TLSCfg struct {
	tlscfg *_Ctype_struct_tlscfg
}

// NewTLSCfg creates a new TLSCfg
func NewTLSCfg() *TLSCfg {
	return &TLSCfg{
		tlscfg: C.dtls_build_tlscfg(),
	}
}

// Fingerprint generates a SHA-256 fingerprint of the certificate
func (t *TLSCfg) Fingerprint() string {
	rawFingerprint := C.dtls_tlscfg_fingerprint(t.tlscfg)
	defer C.free(unsafe.Pointer(rawFingerprint))
	return C.GoString(rawFingerprint)
}

// Close cleans up the associated OpenSSL resources
func (t *TLSCfg) Close() {
	C.dtls_tlscfg_cleanup(t.tlscfg)
}

// State represents all the state needed for a DTLS session
type State struct {
	*TLSCfg
	sslctx         *_Ctype_struct_ssl_ctx_st
	dtlsSession    *_Ctype_struct_dtls_sess
	rawSrc, rawDst *_Ctype_char
}

// NewState creates a new DTLS session
func NewState(tlscfg *TLSCfg, isClient bool, src, dst string) (d *State, err error) {
	if tlscfg == nil || tlscfg.tlscfg == nil {
		return d, errors.Errorf("TLSCfg must not be nil")
	}

	d = &State{
		TLSCfg: tlscfg,
		rawSrc: C.CString(src),
		rawDst: C.CString(dst),
	}

	d.sslctx = C.dtls_build_sslctx(d.tlscfg)
	d.dtlsSession = C.dtls_build_session(d.sslctx, C.bool(!isClient))

	return d, err
}

// Close cleans up the associated OpenSSL resources
func (d *State) Close() {
	C.free(unsafe.Pointer(d.rawSrc))
	C.free(unsafe.Pointer(d.rawDst))
	C.dtls_session_cleanup(d.sslctx, d.dtlsSession)
}

// CertPair is the client+server key and profile extracted for SRTP
type CertPair struct {
	ClientWriteKey []byte
	ServerWriteKey []byte
	Profile        string
}

// MaybeHandleDTLSPacket checks if the packet is a DTLS packet, and if it is passes to the DTLS session
func (d *State) MaybeHandleDTLSPacket(packet []byte, size int) (isDTLSPacket bool, certPair *CertPair) {
	if packet[0] >= 20 && packet[0] <= 64 {
		isDTLSPacket = true
		packetRaw := C.CBytes(packet)
		defer C.free(unsafe.Pointer(packetRaw))

		if ret := C.dtls_handle_incoming(d.dtlsSession, d.rawSrc, d.rawDst, packetRaw, C.int(size)); ret != nil {
			certPair = &CertPair{
				ClientWriteKey: []byte(C.GoStringN(&ret.client_write_key[0], ret.key_length)),
				ServerWriteKey: []byte(C.GoStringN(&ret.server_write_key[0], ret.key_length)),
				Profile:        C.GoString(&ret.profile[0]),
			}
			C.free(unsafe.Pointer(ret))
		}

		return isDTLSPacket, certPair
	}

	return isDTLSPacket, certPair
}

// DoHandshake sends the DTLS handshake it the remote peer
func (d *State) DoHandshake() {
	C.dtls_do_handshake(d.dtlsSession, d.rawSrc, d.rawDst)
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
}
