//go:build opusred && !js

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package opusredtest

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/red"
	"github.com/pion/rtp"
)

// StreamMetadata captures the negotiation fields used at each binding.
type StreamMetadata struct {
	ID                                string                           `json:"id"`
	SSRC                              uint32                           `json:"ssrc"`
	SSRCRetransmission                uint32                           `json:"ssrcRetransmission"`
	SSRCForwardErrorCorrection        uint32                           `json:"ssrcForwardErrorCorrection"`
	PayloadType                       uint8                            `json:"payloadType"`
	PayloadTypeRetransmission         uint8                            `json:"payloadTypeRetransmission"`
	PayloadTypeForwardErrorCorrection uint8                            `json:"payloadTypeForwardErrorCorrection"`
	MimeType                          string                           `json:"mimeType"`
	ClockRate                         uint32                           `json:"clockRate"`
	Channels                          uint16                           `json:"channels"`
	SDPFmtpLine                       string                           `json:"sdpFmtpLine"`
	RTPHeaderExtensions               []interceptor.RTPHeaderExtension `json:"rtpHeaderExtensions"`
	RTCPFeedback                      []interceptor.RTCPFeedback       `json:"rtcpFeedback"`
}

// Binding records original and adapted metadata without exposing mutable StreamInfo.
type Binding struct {
	Factory   string         `json:"factory"`
	Direction string         `json:"direction"`
	Original  StreamMetadata `json:"original"`
	Adapted   StreamMetadata `json:"adapted"`
}

// Evidence separates original media from physical send and receive observations.
type Evidence struct {
	Original []PacketRecord `json:"original"`
	Sent     []WireRecord   `json:"sent"`
	Received []WireRecord   `json:"received"`
	Bindings []Binding      `json:"bindings"`
	Faults   []string       `json:"faults"`
}

// Session owns one trial's immutable mapping and concurrent evidence collection.
type Session struct {
	mutex           sync.Mutex
	mapping         Mapping
	armed           bool
	bindingsStarted bool
	evidence        Evidence
	sender          interceptor.Factory
	receiver        interceptor.Factory
}

// NewSession constructs real RED factories but does not arm stream bindings.
func NewSession(maxPacketSize int) (*Session, error) {
	if maxPacketSize <= 0 {
		return nil, fmt.Errorf("maximum packet size must be positive")
	}
	sender, err := red.NewSenderInterceptor(red.SenderMaxPacketSize(maxPacketSize))
	if err != nil {
		return nil, err
	}
	receiver, err := red.NewReceiverInterceptor()
	if err != nil {
		return nil, err
	}
	return &Session{
		sender: sender, receiver: receiver,
		evidence: Evidence{Original: []PacketRecord{}, Sent: []WireRecord{}, Received: []WireRecord{}, Bindings: []Binding{}, Faults: []string{}},
	}, nil
}

// Arm installs the validated mapping exactly once, before any audio binding.
func (session *Session) Arm(mapping Mapping) error {
	if err := validateMapping(mapping); err != nil {
		return err
	}
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if session.armed || session.bindingsStarted {
		return fmt.Errorf("trial mapping must be armed once before audio bindings")
	}
	session.mapping = mapping
	session.armed = true
	return nil
}

type sessionFactory struct {
	session *Session
	kind    string
}

func (factory sessionFactory) NewInterceptor(id string) (interceptor.Interceptor, error) {
	instance := &sessionInterceptor{session: factory.session, kind: factory.kind}
	var err error
	switch factory.kind {
	case "sender":
		instance.inner, err = factory.session.sender.NewInterceptor(id)
	case "receiver":
		instance.inner, err = factory.session.receiver.NewInterceptor(id)
	}
	return instance, err
}

// WireFactory must be registered first to observe physical RTP at the transport boundary.
func (session *Session) WireFactory() interceptor.Factory {
	return sessionFactory{session: session, kind: "wire"}
}

// SenderFactory adapts binding metadata for the real upstream RED sender.
func (session *Session) SenderFactory() interceptor.Factory {
	return sessionFactory{session: session, kind: "sender"}
}

// ReceiverFactory adapts binding metadata for the real upstream RED receiver.
func (session *Session) ReceiverFactory() interceptor.Factory {
	return sessionFactory{session: session, kind: "receiver"}
}

// SourceFactory must be registered last to snapshot original outgoing Opus media.
func (session *Session) SourceFactory() interceptor.Factory {
	return sessionFactory{session: session, kind: "source"}
}

func metadata(info *interceptor.StreamInfo) StreamMetadata {
	return StreamMetadata{
		ID: info.ID, SSRC: info.SSRC, SSRCRetransmission: info.SSRCRetransmission,
		SSRCForwardErrorCorrection: info.SSRCForwardErrorCorrection, PayloadType: info.PayloadType,
		PayloadTypeRetransmission:         info.PayloadTypeRetransmission,
		PayloadTypeForwardErrorCorrection: info.PayloadTypeForwardErrorCorrection,
		MimeType:                          info.MimeType, ClockRate: info.ClockRate, Channels: info.Channels, SDPFmtpLine: info.SDPFmtpLine,
		RTPHeaderExtensions: append([]interceptor.RTPHeaderExtension(nil), info.RTPHeaderExtensions...),
		RTCPFeedback:        append([]interceptor.RTCPFeedback(nil), info.RTCPFeedback...),
	}
}

func copyInfo(info *interceptor.StreamInfo) *interceptor.StreamInfo {
	if info == nil {
		return nil
	}
	cloned := *info
	cloned.RTPHeaderExtensions = append([]interceptor.RTPHeaderExtension(nil), info.RTPHeaderExtensions...)
	cloned.RTCPFeedback = append([]interceptor.RTCPFeedback(nil), info.RTCPFeedback...)
	if info.Attributes != nil {
		cloned.Attributes = make(interceptor.Attributes, len(info.Attributes))
		for key, value := range info.Attributes {
			cloned.Attributes[key] = value
		}
	}
	return &cloned
}

func adaptedInfo(info *interceptor.StreamInfo, mapping Mapping) *interceptor.StreamInfo {
	adapted := copyInfo(info)
	if adapted == nil {
		return nil
	}
	adapted.MimeType = "audio/opus"
	adapted.PayloadType = mapping.OpusPT
	adapted.PayloadTypeForwardErrorCorrection = 0
	if mapping.HasRED {
		adapted.PayloadTypeForwardErrorCorrection = mapping.REDPT
	}
	adapted.ClockRate, adapted.Channels, adapted.SDPFmtpLine = mapping.ClockRate, mapping.Channels, mapping.OpusFMTP
	return adapted
}

func (session *Session) bind(info *interceptor.StreamInfo, kind, direction string) (Mapping, *interceptor.StreamInfo, error) {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if info == nil {
		err := fmt.Errorf("%s %s binding has nil StreamInfo", kind, direction)
		session.evidence.Faults = append(session.evidence.Faults, err.Error())
		return Mapping{}, nil, err
	}
	if !strings.HasPrefix(strings.ToLower(info.MimeType), "audio/") {
		return Mapping{}, copyInfo(info), nil
	}
	session.bindingsStarted = true
	if !session.armed {
		err := fmt.Errorf("%s %s audio binding before mapping was armed", kind, direction)
		session.evidence.Faults = append(session.evidence.Faults, err.Error())
		return Mapping{}, nil, err
	}
	adapted := copyInfo(info)
	if kind == "sender" || kind == "receiver" {
		adapted = adaptedInfo(info, session.mapping)
	}
	session.evidence.Bindings = append(session.evidence.Bindings, Binding{
		Factory: kind, Direction: direction, Original: metadata(info), Adapted: metadata(adapted),
	})
	return session.mapping, adapted, nil
}

func (session *Session) fault(err error) {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	session.evidence.Faults = append(session.evidence.Faults, err.Error())
}

func (session *Session) wire(header *rtp.Header, payload []byte, mapping Mapping, direction string) error {
	record, err := ParseWire(header, payload, mapping)
	if err != nil {
		session.fault(fmt.Errorf("%s wire: %w", direction, err))
		return err
	}
	session.mutex.Lock()
	defer session.mutex.Unlock()
	if direction == "sent" {
		session.evidence.Sent = append(session.evidence.Sent, record)
	} else {
		session.evidence.Received = append(session.evidence.Received, record)
	}
	return nil
}

type sessionInterceptor struct {
	interceptor.NoOp
	session *Session
	kind    string
	inner   interceptor.Interceptor
}

func (instance *sessionInterceptor) BindLocalStream(info *interceptor.StreamInfo, writer interceptor.RTPWriter) interceptor.RTPWriter {
	mapping, adapted, err := instance.session.bind(info, instance.kind, "local")
	if err != nil {
		return interceptor.RTPWriterFunc(func(*rtp.Header, []byte, interceptor.Attributes) (int, error) { return 0, err })
	}
	if instance.inner != nil {
		return instance.inner.BindLocalStream(adapted, writer)
	}
	if info == nil || !strings.HasPrefix(strings.ToLower(info.MimeType), "audio/") {
		return writer
	}
	return interceptor.RTPWriterFunc(func(header *rtp.Header, payload []byte, attributes interceptor.Attributes) (int, error) {
		if instance.kind == "wire" {
			if err := instance.session.wire(header, payload, mapping, "sent"); err != nil {
				return 0, err
			}
		} else {
			if header == nil || header.PayloadType != mapping.OpusPT {
				err := fmt.Errorf("original source packet lacks negotiated Opus header")
				instance.session.fault(err)
				return 0, err
			}
			record := NewPacketRecord(header, payload)
			instance.session.mutex.Lock()
			instance.session.evidence.Original = append(instance.session.evidence.Original, record)
			instance.session.mutex.Unlock()
		}
		return writer.Write(header, payload, attributes)
	})
}

func (instance *sessionInterceptor) BindRemoteStream(info *interceptor.StreamInfo, reader interceptor.RTPReader) interceptor.RTPReader {
	mapping, adapted, err := instance.session.bind(info, instance.kind, "remote")
	if err != nil {
		return interceptor.RTPReaderFunc(func([]byte, interceptor.Attributes) (int, interceptor.Attributes, error) { return 0, nil, err })
	}
	if instance.inner != nil {
		return instance.inner.BindRemoteStream(adapted, reader)
	}
	if instance.kind != "wire" || info == nil || !strings.HasPrefix(strings.ToLower(info.MimeType), "audio/") {
		return reader
	}
	return interceptor.RTPReaderFunc(func(buffer []byte, attributes interceptor.Attributes) (int, interceptor.Attributes, error) {
		n, returned, err := reader.Read(buffer, attributes)
		if err != nil {
			return n, returned, err
		}
		if n < 0 || n > len(buffer) {
			err := fmt.Errorf("physical reader returned invalid length %d", n)
			instance.session.fault(err)
			return 0, returned, err
		}
		var packet rtp.Packet
		if err := packet.Unmarshal(buffer[:n]); err != nil {
			instance.session.fault(err)
			return 0, returned, err
		}
		if err := instance.session.wire(&packet.Header, packet.Payload, mapping, "received"); err != nil {
			return 0, returned, err
		}
		return n, returned, nil
	})
}

func (instance *sessionInterceptor) UnbindLocalStream(info *interceptor.StreamInfo) {
	if instance.inner != nil {
		instance.session.mutex.Lock()
		mapping := instance.session.mapping
		instance.session.mutex.Unlock()
		instance.inner.UnbindLocalStream(adaptedInfo(info, mapping))
	}
}
func (instance *sessionInterceptor) UnbindRemoteStream(info *interceptor.StreamInfo) {
	if instance.inner != nil {
		instance.session.mutex.Lock()
		mapping := instance.session.mapping
		instance.session.mutex.Unlock()
		instance.inner.UnbindRemoteStream(adaptedInfo(info, mapping))
	}
}
func (instance *sessionInterceptor) Close() error {
	if instance.inner != nil {
		return instance.inner.Close()
	}
	return nil
}

func cloneWire(records []WireRecord) []WireRecord {
	cloned := append([]WireRecord{}, records...)
	for index := range cloned {
		cloned[index].Blocks = append([]BlockRecord{}, records[index].Blocks...)
	}
	return cloned
}
func cloneMetadata(value StreamMetadata) StreamMetadata {
	value.RTPHeaderExtensions = append([]interceptor.RTPHeaderExtension(nil), value.RTPHeaderExtensions...)
	value.RTCPFeedback = append([]interceptor.RTCPFeedback(nil), value.RTCPFeedback...)
	return value
}

// Snapshot returns evidence whose slices are owned by the caller.
func (session *Session) Snapshot() Evidence {
	session.mutex.Lock()
	defer session.mutex.Unlock()
	result := Evidence{
		Original: append([]PacketRecord{}, session.evidence.Original...), Sent: cloneWire(session.evidence.Sent), Received: cloneWire(session.evidence.Received),
		Bindings: append([]Binding{}, session.evidence.Bindings...), Faults: append([]string{}, session.evidence.Faults...),
	}
	for index := range result.Bindings {
		result.Bindings[index].Original = cloneMetadata(result.Bindings[index].Original)
		result.Bindings[index].Adapted = cloneMetadata(result.Bindings[index].Adapted)
	}
	return result
}
