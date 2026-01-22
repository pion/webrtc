// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build js && wasm
// +build js,wasm

package webrtc

import "syscall/js"

// DTLSTransport allows an application access to information about the DTLS
// transport over which RTP and RTCP packets are sent and received by
// RTPSender and RTPReceiver, as well other data such as SCTP packets sent
// and received by data channels.
type DTLSTransport struct {
	// Pointer to the underlying JavaScript DTLSTransport object.
	underlying js.Value
}

// JSValue returns the underlying RTCDtlsTransport
func (r *DTLSTransport) JSValue() js.Value {
	return r.underlying
}

// ICETransport returns the currently-configured *ICETransport or nil
// if one has not been configured
func (r *DTLSTransport) ICETransport() *ICETransport {
	underlying := r.underlying.Get("iceTransport")
	if underlying.IsNull() || underlying.IsUndefined() {
		return nil
	}

	return &ICETransport{
		underlying: underlying,
	}
}

func (t *DTLSTransport) GetRemoteCertificate() []byte {
	if t.underlying.IsNull() || t.underlying.IsUndefined() {
		return nil
	}

	// Firefox does not support getRemoteCertificates: https://bugzilla.mozilla.org/show_bug.cgi?id=1805446
	jsGet := t.underlying.Get("getRemoteCertificates")
	if jsGet.IsUndefined() || jsGet.IsNull() {
		return nil
	}

	jsCerts := t.underlying.Call("getRemoteCertificates")
	if jsCerts.Length() == 0 {
		return nil
	}

	buf := jsCerts.Index(0)
	u8 := js.Global().Get("Uint8Array").New(buf)

	if u8.Length() == 0 {
		return nil
	}

	cert := make([]byte, u8.Length())
	js.CopyBytesToGo(cert, u8)

	return cert
}
