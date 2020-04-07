// +build !js

package webrtc

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatsTimestampTime(t *testing.T) {
	for _, test := range []struct {
		Timestamp StatsTimestamp
		WantTime  time.Time
	}{
		{
			Timestamp: 0,
			WantTime:  time.Unix(0, 0),
		},
		{
			Timestamp: 1,
			WantTime:  time.Unix(0, 1e6),
		},
		{
			Timestamp: 0.001,
			WantTime:  time.Unix(0, 1e3),
		},
	} {
		if got, want := test.Timestamp.Time(), test.WantTime.UTC(); got != want {
			t.Fatalf("StatsTimestamp(%v).Time() = %v, want %v", test.Timestamp, got, want)
		}
	}
}

// TODO(maxhawkins): replace with a more meaningful test
func TestStatsMarshal(t *testing.T) {
	for _, test := range []Stats{
		AudioReceiverStats{},
		AudioSenderStats{},
		CertificateStats{},
		CodecStats{},
		DataChannelStats{},
		ICECandidatePairStats{},
		ICECandidateStats{},
		InboundRTPStreamStats{},
		MediaStreamStats{},
		OutboundRTPStreamStats{},
		PeerConnectionStats{},
		RemoteInboundRTPStreamStats{},
		RemoteOutboundRTPStreamStats{},
		RTPContributingSourceStats{},
		SenderAudioTrackAttachmentStats{},
		SenderAudioTrackAttachmentStats{},
		SenderVideoTrackAttachmentStats{},
		TransportStats{},
		VideoReceiverStats{},
		VideoReceiverStats{},
		VideoSenderStats{},
	} {
		_, err := json.Marshal(test)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func waitWithTimeout(t *testing.T, wg *sync.WaitGroup) {
	// Wait for all of the event handlers to be triggered.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		done <- struct{}{}
	}()
	timeout := time.After(5 * time.Second)
	select {
	case <-done:
		break
	case <-timeout:
		t.Fatal("timed out waiting for waitgroup")
	}
}

func getConnectionStats(t *testing.T, report StatsReport, pc *PeerConnection) PeerConnectionStats {
	stats, ok := report.GetConnectionStats(pc)
	assert.True(t, ok)
	assert.Equal(t, stats.Type, StatsTypePeerConnection)
	return stats
}

func getDataChannelStats(t *testing.T, report StatsReport, dc *DataChannel) DataChannelStats {
	stats, ok := report.GetDataChannelStats(dc)
	assert.True(t, ok)
	assert.Equal(t, stats.Type, StatsTypeDataChannel)
	return stats
}

func getTransportStats(t *testing.T, report StatsReport, statsID string) TransportStats {
	stats, ok := report[statsID]
	assert.True(t, ok)
	transportStats, ok := stats.(TransportStats)
	assert.True(t, ok)
	assert.Equal(t, transportStats.Type, StatsTypeTransport)
	return transportStats
}

func findLocalCandidateStats(report StatsReport) []ICECandidateStats {
	result := []ICECandidateStats{}
	for _, s := range report {
		stats, ok := s.(ICECandidateStats)
		if ok && stats.Type == StatsTypeLocalCandidate {
			result = append(result, stats)
		}
	}
	return result
}

func findRemoteCandidateStats(report StatsReport) []ICECandidateStats {
	result := []ICECandidateStats{}
	for _, s := range report {
		stats, ok := s.(ICECandidateStats)
		if ok && stats.Type == StatsTypeRemoteCandidate {
			result = append(result, stats)
		}
	}
	return result
}

func findCandidatePairStats(t *testing.T, report StatsReport) []ICECandidatePairStats {
	result := []ICECandidatePairStats{}
	for _, s := range report {
		stats, ok := s.(ICECandidatePairStats)
		if ok {
			assert.Equal(t, StatsTypeCandidatePair, stats.Type)
			result = append(result, stats)
		}
	}
	return result
}

func signalPairForStats(pcOffer *PeerConnection, pcAnswer *PeerConnection) error {
	offerChan := make(chan SessionDescription)
	pcOffer.OnICECandidate(func(candidate *ICECandidate) {
		if candidate == nil {
			offerChan <- *pcOffer.PendingLocalDescription()
		}
	})

	offer, err := pcOffer.CreateOffer(nil)
	if err != nil {
		return err
	}
	if err := pcOffer.SetLocalDescription(offer); err != nil {
		return err
	}

	timeout := time.After(3 * time.Second)
	select {
	case <-timeout:
		return fmt.Errorf("timed out waiting to receive offer")
	case offer := <-offerChan:
		if err := pcAnswer.SetRemoteDescription(offer); err != nil {
			return err
		}

		answer, err := pcAnswer.CreateAnswer(nil)
		if err != nil {
			return err
		}

		if err = pcAnswer.SetLocalDescription(answer); err != nil {
			return err
		}

		err = pcOffer.SetRemoteDescription(answer)
		if err != nil {
			return err
		}
		return nil
	}
}

func TestPeerConnection_GetStats(t *testing.T) {
	offerPC, answerPC, err := newPair()
	assert.NoError(t, err)

	baseLineReportPCOffer := offerPC.GetStats()
	baseLineReportPCAnswer := answerPC.GetStats()

	connStatsOffer := getConnectionStats(t, baseLineReportPCOffer, offerPC)
	connStatsAnswer := getConnectionStats(t, baseLineReportPCAnswer, answerPC)

	for _, connStats := range []PeerConnectionStats{connStatsOffer, connStatsAnswer} {
		assert.Equal(t, uint32(0), connStats.DataChannelsOpened)
		assert.Equal(t, uint32(0), connStats.DataChannelsClosed)
		assert.Equal(t, uint32(0), connStats.DataChannelsRequested)
		assert.Equal(t, uint32(0), connStats.DataChannelsAccepted)
	}

	// Create a DC, open it and send a message
	offerDC, err := offerPC.CreateDataChannel("offerDC", nil)
	assert.NoError(t, err)

	msg := []byte("a classic test message")
	offerDC.OnOpen(func() {
		assert.NoError(t, offerDC.Send(msg))
	})

	dcWait := sync.WaitGroup{}
	dcWait.Add(1)

	answerDCChan := make(chan *DataChannel)
	answerPC.OnDataChannel(func(d *DataChannel) {
		d.OnOpen(func() {
			answerDCChan <- d
		})
		d.OnMessage(func(m DataChannelMessage) {
			dcWait.Done()
		})
	})

	assert.NoError(t, signalPairForStats(offerPC, answerPC))
	waitWithTimeout(t, &dcWait)

	answerDC := <-answerDCChan

	reportPCOffer := offerPC.GetStats()
	reportPCAnswer := answerPC.GetStats()

	connStatsOffer = getConnectionStats(t, reportPCOffer, offerPC)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsOpened)
	assert.Equal(t, uint32(0), connStatsOffer.DataChannelsClosed)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsRequested)
	assert.Equal(t, uint32(0), connStatsOffer.DataChannelsAccepted)
	dcStatsOffer := getDataChannelStats(t, reportPCOffer, offerDC)
	assert.Equal(t, DataChannelStateOpen, dcStatsOffer.State)
	assert.Equal(t, uint32(1), dcStatsOffer.MessagesSent)
	assert.Equal(t, uint64(len(msg)), dcStatsOffer.BytesSent)
	assert.NotEmpty(t, findLocalCandidateStats(reportPCOffer))
	assert.NotEmpty(t, findRemoteCandidateStats(reportPCOffer))
	assert.NotEmpty(t, findCandidatePairStats(t, reportPCOffer))

	connStatsAnswer = getConnectionStats(t, reportPCAnswer, answerPC)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsOpened)
	assert.Equal(t, uint32(0), connStatsAnswer.DataChannelsClosed)
	assert.Equal(t, uint32(0), connStatsAnswer.DataChannelsRequested)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsAccepted)
	dcStatsAnswer := getDataChannelStats(t, reportPCAnswer, answerDC)
	assert.Equal(t, DataChannelStateOpen, dcStatsAnswer.State)
	assert.Equal(t, uint32(1), dcStatsAnswer.MessagesReceived)
	assert.Equal(t, uint64(len(msg)), dcStatsAnswer.BytesReceived)
	assert.NotEmpty(t, findLocalCandidateStats(reportPCAnswer))
	assert.NotEmpty(t, findRemoteCandidateStats(reportPCAnswer))
	assert.NotEmpty(t, findCandidatePairStats(t, reportPCAnswer))

	// Close answer DC now
	dcWait = sync.WaitGroup{}
	dcWait.Add(1)
	offerDC.OnClose(func() {
		dcWait.Done()
	})
	assert.NoError(t, answerDC.Close())
	waitWithTimeout(t, &dcWait)
	time.Sleep(10 * time.Millisecond)

	reportPCOffer = offerPC.GetStats()
	reportPCAnswer = answerPC.GetStats()

	connStatsOffer = getConnectionStats(t, reportPCOffer, offerPC)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsOpened)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsClosed)
	assert.Equal(t, uint32(1), connStatsOffer.DataChannelsRequested)
	assert.Equal(t, uint32(0), connStatsOffer.DataChannelsAccepted)
	dcStatsOffer = getDataChannelStats(t, reportPCOffer, offerDC)
	assert.Equal(t, DataChannelStateClosed, dcStatsOffer.State)

	connStatsAnswer = getConnectionStats(t, reportPCAnswer, answerPC)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsOpened)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsClosed)
	assert.Equal(t, uint32(0), connStatsAnswer.DataChannelsRequested)
	assert.Equal(t, uint32(1), connStatsAnswer.DataChannelsAccepted)
	dcStatsAnswer = getDataChannelStats(t, reportPCAnswer, answerDC)
	assert.Equal(t, DataChannelStateClosed, dcStatsAnswer.State)

	answerICETransportStats := getTransportStats(t, reportPCAnswer, "iceTransport")
	offerICETransportStats := getTransportStats(t, reportPCOffer, "iceTransport")
	assert.GreaterOrEqual(t, offerICETransportStats.BytesSent, answerICETransportStats.BytesReceived)
	assert.GreaterOrEqual(t, answerICETransportStats.BytesSent, offerICETransportStats.BytesReceived)

	answerSCTPTransportStats := getTransportStats(t, reportPCAnswer, "sctpTransport")
	offerSCTPTransportStats := getTransportStats(t, reportPCOffer, "sctpTransport")
	assert.GreaterOrEqual(t, offerSCTPTransportStats.BytesSent, answerSCTPTransportStats.BytesReceived)
	assert.GreaterOrEqual(t, answerSCTPTransportStats.BytesSent, offerSCTPTransportStats.BytesReceived)

	assert.NoError(t, offerPC.Close())
	assert.NoError(t, answerPC.Close())
}

func TestPeerConnection_GetStats_Closed(t *testing.T) {
	pc, err := NewPeerConnection(Configuration{})
	assert.NoError(t, err)

	assert.NoError(t, pc.Close())

	pc.GetStats()
}
