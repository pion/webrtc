package sdp

// MediaDescription represents a media type.  Currently defined media are "audio",
// "video", "text", "application", and "message", although this list
// may be extended in the future
// https://tools.ietf.org/html/rfc4566#section-5.14
type MediaDescription struct {
	// MediaName is m=<media> <port> <proto> <fmt>
	// <media> is the media type
	// <port> is the transport port to which the media stream is sent
	// <proto> is the transport protocol
	// <fmt> is a media format description
	// https://tools.ietf.org/html/rfc4566#section-5.13
	MediaName string

	// SessionInformation field provides textual information about the session.  There
	// MUST be at most one session-level SessionInformation field per session description,
	// and at most one SessionInformation field per media
	// https://tools.ietf.org/html/rfc4566#section-5.4
	MediaInformation string

	// ConnectionData a session description MUST contain either at least one ConnectionData field in
	// each media description or a single ConnectionData field at the session level.
	// https://tools.ietf.org/html/rfc4566#section-5.7
	ConnectionData string

	// Bandwidth field denotes the proposed bandwidth to be used by the
	// session or media
	// b=<bwtype>:<bandwidth>
	// https://tools.ietf.org/html/rfc4566#section-5.8
	Bandwidth []string

	// EncryptionKeys if for when the SessionDescription is transported over a secure and trusted channel,
	// the Session Description Protocol MAY be used to convey encryption keys
	// https://tools.ietf.org/html/rfc4566#section-5.11
	EncryptionKeys []string

	// Attributes are the primary means for extending SDP.  Attributes may
	// be defined to be used as "session-level" attributes, "media-level"
	// attributes, or both.
	// https://tools.ietf.org/html/rfc4566#section-5.12
	Attributes []string
}
