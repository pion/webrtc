package main

import (
	"fmt"
	"net"
	"strconv"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/internal/dtls"

	"golang.org/x/net/ipv4"
)

func packetHandler(conn *ipv4.PacketConn, srcString string, remoteKey [16]byte) {
	const MTU = 8192
	buffer := make([]byte, MTU)

	var d *dtls.DTLSState
	for {
		n, _, srcAddr, _ := conn.ReadFrom(buffer)

		if d != nil && d.HandleDTLSPacket(buffer, n) {
			fmt.Println("Handling DTLS")
		}

		if packetType, err := stun.GetPacketType(buffer[:n]); err == nil && packetType == stun.PacketTypeSTUN {
			if d == nil {
				d, err = dtls.New(true, srcString, srcAddr.String())
				if err != nil {
					fmt.Println(err)
				} else {
					d.DoHandshake()
					fmt.Println("sending handshake")
				}
			} else {
				d.DoHandshake()
				fmt.Println("sending handshake")
				return
			}
			return
			if m, err := stun.NewMessage(buffer[:n]); err == nil && m.Class == stun.ClassRequest && m.Method == stun.MethodBinding {
				dstAddr := &stun.TransportAddr{IP: srcAddr.(*net.UDPAddr).IP, Port: srcAddr.(*net.UDPAddr).Port}
				err := stun.BuildAndSend(conn, dstAddr, stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
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
				)
				if err != nil {
					fmt.Println(err)
				}

			}
		}

	}

}

func udpListener(ip string, remoteKey [16]byte) (int, error) {
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
	go packetHandler(conn, srcString, remoteKey)
	return addr.Port, err
}
