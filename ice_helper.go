package main

import (
	"fmt"
	"net"
	"strconv"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"

	"golang.org/x/net/ipv4"
)

func packetHandler(conn *ipv4.PacketConn, srcString string, remoteKey []byte, tlscfg *dtls.TLSCfg) {
	const MTU = 8192
	buffer := make([]byte, MTU)

	dtlsStates := make(map[string]*dtls.DTLSState)
	for {
		n, _, rawDstAddr, _ := conn.ReadFrom(buffer)
		d, haveHandshaked := dtlsStates[rawDstAddr.String()]

		if haveHandshaked {
			if handled, certPair := d.MaybeHandleDTLSPacket(buffer, n); handled {
				if certPair != nil {
					fmt.Println(certPair)
					fmt.Println(len(certPair.ServerWriteKey))
				}
				fmt.Println("Handled DTLS")
				continue
			}
		}

		if packetType, err := stun.GetPacketType(buffer[:n]); err == nil && packetType == stun.PacketTypeSTUN {
			if m, err := stun.NewMessage(buffer[:n]); err == nil && m.Class == stun.ClassRequest && m.Method == stun.MethodBinding {
				dstAddr := &stun.TransportAddr{IP: rawDstAddr.(*net.UDPAddr).IP, Port: rawDstAddr.(*net.UDPAddr).Port}
				if err := stun.BuildAndSend(conn, dstAddr, stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
					&stun.XorMappedAddress{
						XorAddress: stun.XorAddress{
							IP:   dstAddr.IP,
							Port: dstAddr.Port,
						},
					},
					&stun.MessageIntegrity{
						Key: remoteKey,
					},
					&stun.Fingerprint{},
				); err != nil {
					fmt.Println(err)
				}
			}
		} else {
			fmt.Println("Probably SRTP")
		}

		if !haveHandshaked {
			d, err := dtls.NewDTLSState(tlscfg, true, srcString, rawDstAddr.String())
			if err != nil {
				fmt.Println(err)
				continue
			}

			d.DoHandshake()
			dtlsStates[rawDstAddr.String()] = d
		}
	}
}

func udpListener(ip string, remoteKey []byte, tlscfg *dtls.TLSCfg) (int, error) {
	listener, err := net.ListenPacket("udp4", ip+":0")
	if err != nil {
		return 0, err
	}

	conn := ipv4.NewPacketConn(listener)
	err = conn.SetControlMessage(ipv4.FlagDst, true)
	if err != nil {
		return 0, err
	}

	addr, err := stun.NewTransportAddr(listener.LocalAddr())

	srcString := ip + ":" + strconv.Itoa(addr.Port)

	dtls.AddListener(srcString, conn)
	go packetHandler(conn, srcString, remoteKey, tlscfg)
	return addr.Port, err
}
