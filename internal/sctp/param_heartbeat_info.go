package sctp

type ParamHeartbeatInfo struct {
	ParamHeader
	HeartbeatInformation []byte
}

func (h *ParamHeartbeatInfo) Marshal() ([]byte, error) {
	h.typ = HeartbeatInfo
	h.raw = h.HeartbeatInformation
	return h.ParamHeader.Marshal()
}

func (h *ParamHeartbeatInfo) Unmarshal(raw []byte) (Param, error) {
	h.ParamHeader.Unmarshal(raw)
	h.HeartbeatInformation = h.raw
	return h, nil
}
