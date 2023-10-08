// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

package webrtc

import "github.com/pion/dtls/v2"

const (
	// Unknown defines default public constant to use for "enum" like struct
	// comparisons when no value was defined.
	Unknown    = iota
	unknownStr = "unknown"

	// Equal to UDP MTU
	receiveMTU = 1460

	// simulcastProbeCount is the amount of RTP Packets
	// that handleUndeclaredSSRC will read and try to dispatch from
	// mid and rid values
	simulcastProbeCount = 10

	// simulcastMaxProbeRoutines is how many active routines can be used to probe
	// If the total amount of incoming SSRCes exceeds this new requests will be ignored
	simulcastMaxProbeRoutines = 25

	mediaSectionApplication = "application"

	sdpAttributeRid = "rid"

	sdpAttributeSimulcast = "simulcast"

	rtpOutboundMTU = 1200

	rtpPayloadTypeBitmask = 0x7F

	incomingUnhandledRTPSsrc = "Incoming unhandled RTP ssrc(%d), OnTrack will not be fired. %v"

	generatedCertificateOrigin = "WebRTC"

	sdesRepairRTPStreamIDURI = "urn:ietf:params:rtp-hdrext:sdes:repaired-rtp-stream-id"

	// AttributeRtxPayloadType is the interceptor attribute added when Read() returns an RTX packet containing the RTX stream payload type
	AttributeRtxPayloadType = "rtx_payload_type"
	// AttributeRtxSsrc is the interceptor attribute added when Read() returns an RTX packet containing the RTX stream SSRC
	AttributeRtxSsrc = "rtx_ssrc"
	// AttributeRtxSequenceNumber is the interceptor attribute added when Read() returns an RTX packet containing the RTX stream sequence number
	AttributeRtxSequenceNumber = "rtx_sequence_number"
)

func defaultSrtpProtectionProfiles() []dtls.SRTPProtectionProfile {
	return []dtls.SRTPProtectionProfile{dtls.SRTP_AEAD_AES_256_GCM, dtls.SRTP_AEAD_AES_128_GCM, dtls.SRTP_AES128_CM_HMAC_SHA1_80}
}
