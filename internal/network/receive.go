package network

import (
	"fmt"

	"github.com/pions/webrtc/internal/sctp"
	"github.com/pkg/errors"
)

const receiveMTU = 8192

// TODO: Move to SCTP Conn
func (m *Manager) handleSCTP(raw []byte, a *sctp.Association) {
	m.sctpAssociation.Lock()
	defer m.sctpAssociation.Unlock()

	if err := a.HandleInbound(raw); err != nil {
		fmt.Println(errors.Wrap(err, "Failed to push SCTP packet"))
	}
}

// TODO: Continue to phase out the networkLoop.
func (m *Manager) networkLoop() {
	buffer := make([]byte, receiveMTU)
	for {
		n, err := m.dtlsConn.Read(buffer)

		if err != nil {
			fmt.Println("NetworkLoop failed to read", err)
			return
		}

		m.handleSCTP(buffer[:n], m.sctpAssociation)
	}
}
