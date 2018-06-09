package dtls

/*
#cgo CFLAGS: -I .
#cgo LDFLAGS: -lcrypto -lssl

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

var listenerMap map[string]*ipv4.PacketConn = make(map[string]*ipv4.PacketConn)
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
		strIp, strPort, err := net.SplitHostPort(dst)
		if err != nil {
			fmt.Println(err)
			return
		}
		port, err := strconv.Atoi(strPort)
		if err != nil {
			fmt.Println(err)
			return
		}
		_, err = conn.WriteTo(buf, nil, &net.UDPAddr{IP: net.ParseIP(strIp), Port: port})
		if err != nil {
			fmt.Println(err)
		}
	} else {
		fmt.Printf("Could not find ipv4.PacketConn for %s \n", src)
	}
}

type TLSCfg struct {
	tlscfg *_Ctype_struct_tlscfg
}

func NewTLSCfg() *TLSCfg {
	return &TLSCfg{
		tlscfg: C.dtls_build_tlscfg(),
	}
}

func (t *TLSCfg) Fingerprint() string {
	rawFingerprint := C.dtls_tlscfg_fingerprint(t.tlscfg)
	defer C.free(unsafe.Pointer(rawFingerprint))
	return C.GoString(rawFingerprint)
}

func (t *TLSCfg) Close() {
	C.dtls_tlscfg_cleanup(t.tlscfg)
}

type DTLSState struct {
	*TLSCfg
	sslctx         *_Ctype_struct_ssl_ctx_st
	dtls_session   *_Ctype_struct_dtls_sess
	rawSrc, rawDst *_Ctype_char
}

func NewDTLSState(tlscfg *TLSCfg, isClient bool, src, dst string) (d *DTLSState, err error) {
	if tlscfg == nil || tlscfg.tlscfg == nil {
		return d, errors.Errorf("TLSCfg must not be nil")
	}

	d = &DTLSState{
		TLSCfg: tlscfg,
		rawSrc: C.CString(src),
		rawDst: C.CString(dst),
	}

	d.sslctx = C.dtls_build_sslctx(d.tlscfg)
	d.dtls_session = C.dtls_build_session(d.sslctx, C.bool(!isClient))

	return d, err
}

func (d *DTLSState) Close() {
	C.free(unsafe.Pointer(d.rawSrc))
	C.free(unsafe.Pointer(d.rawDst))
	C.dtls_session_cleanup(d.sslctx, d.dtls_session)
}

type DTLSCertPair struct {
	ClientWriteKey []byte
	ServerWriteKey []byte
	Profile        string
}

func (d *DTLSState) MaybeHandleDTLSPacket(packet []byte, size int) (isDTLSPacket bool, certPair *DTLSCertPair) {
	if packet[0] >= 20 && packet[0] <= 64 {
		isDTLSPacket = true
		packetRaw := C.CBytes(packet)
		defer C.free(unsafe.Pointer(packetRaw))

		if ret := C.dtls_handle_incoming(d.dtls_session, d.rawSrc, d.rawDst, packetRaw, C.int(size)); ret != nil {
			certPair = &DTLSCertPair{
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

func (d *DTLSState) DoHandshake() {
	C.dtls_do_handshake(d.dtls_session, d.rawSrc, d.rawDst)
}

func AddListener(src string, conn *ipv4.PacketConn) {
	listenerMapLock.Lock()
	listenerMap[src] = conn
	listenerMapLock.Unlock()
}

func RemoveListener(src string) {
}
