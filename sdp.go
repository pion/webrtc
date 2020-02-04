package webrtc

import (
	"strconv"
	"strings"

	"github.com/pion/logging"
	"github.com/pion/sdp/v2"
)

type trackDetails struct {
	kind  RTPCodecType
	label string
	id    string
	ssrc  uint32
}

// extract all trackDetails from an SDP.
func trackDetailsFromSDP(log logging.LeveledLogger, s *sdp.SessionDescription) map[uint32]trackDetails {
	incomingTracks := map[uint32]trackDetails{}

	for _, media := range s.MediaDescriptions {
		for _, attr := range media.Attributes {
			codecType := NewRTPCodecType(media.MediaName.Media)
			if codecType == 0 {
				continue
			}

			if attr.Key == sdp.AttrKeySSRC {
				split := strings.Split(attr.Value, " ")
				ssrc, err := strconv.ParseUint(split[0], 10, 32)
				if err != nil {
					log.Warnf("Failed to parse SSRC: %v", err)
					continue
				}

				trackID := ""
				trackLabel := ""
				if len(split) == 3 && strings.HasPrefix(split[1], "msid:") {
					trackLabel = split[1][len("msid:"):]
					trackID = split[2]
				}

				incomingTracks[uint32(ssrc)] = trackDetails{codecType, trackLabel, trackID, uint32(ssrc)}
				if trackID != "" && trackLabel != "" {
					break // Remote provided Label+ID, we have all the information we need
				}
			}
		}
	}

	return incomingTracks
}
