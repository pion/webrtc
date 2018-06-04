package dtls

/*
#cgo CFLAGS: -I .
#cgo LDFLAGS: -lcrypto -lssl

#include "dtls.h"

*/
import "C"
import (
	"fmt"
	"io/ioutil"
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

var webrtcPacketMTU int = 8192
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

type DTLSState struct {
	tlscfg          *_Ctype_struct_tlscfg
	sslctx          *_Ctype_struct_ssl_ctx_st
	dtls_session    *_Ctype_struct_dtls_sess
	rawSrc, rawDst  *_Ctype_char
	keyRaw, certRaw unsafe.Pointer
}

func New(isClient bool, src, dst string) (d *DTLSState, err error) {
	cert, err := ioutil.ReadFile("domain.crt")
	if err != nil {
		return d, err
	}

	key, err := ioutil.ReadFile("domain.key")
	if err != nil {
		return d, err
	}

	d = &DTLSState{
		rawSrc:  C.CString(src),
		rawDst:  C.CString(dst),
		certRaw: C.CBytes(cert),
		keyRaw:  C.CBytes(key),
	}

	d.tlscfg = C.dtls_build_tlscfg(d.certRaw, C.int(len(cert)), d.keyRaw, C.int(len(key)))
	d.sslctx = C.dtls_build_sslctx(d.tlscfg)
	d.dtls_session = C.dtls_build_session(d.sslctx, C.bool(!isClient))

	return d, err
}

func (d *DTLSState) Close() {
	C.free(unsafe.Pointer(d.certRaw))
	C.free(unsafe.Pointer(d.keyRaw))
	C.free(unsafe.Pointer(d.rawSrc))
	C.free(unsafe.Pointer(d.rawDst))
	C.dtls_session_cleanup(d.tlscfg, d.sslctx, d.dtls_session)
}

func (d *DTLSState) HandleDTLSPacket(packet []byte, size int) bool {
	if packet[0] >= 20 && packet[0] <= 64 {
		packetRaw := C.CBytes(packet)
		C.dtls_handle_incoming(d.dtls_session, d.rawSrc, d.rawDst, packetRaw, C.int(size))
		C.free(unsafe.Pointer(packetRaw))
		return true
	}

	return false
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
