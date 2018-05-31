package main

import (
	"fmt"
	"net"

	"github.com/pions/pkg/stun"
	"golang.org/x/net/ipv4"
)

func packetHandler(relaySocket *ipv4.PacketConn, remoteKey [16]byte) {
	const MTU = 1500
	buffer := make([]byte, MTU)

	for {
		n, _, srcAddr, _ := relaySocket.ReadFrom(buffer)

		if packetType, err := stun.GetPacketType(buffer[:n]); err == nil && packetType == stun.PacketTypeSTUN {
			if m, err := stun.NewMessage(buffer[:n]); err == nil && m.Class == stun.ClassRequest && m.Method == stun.MethodBinding {
				dstAddr := &stun.TransportAddr{IP: srcAddr.(*net.UDPAddr).IP, Port: srcAddr.(*net.UDPAddr).Port}
				err := stun.BuildAndSend(relaySocket, dstAddr, stun.ClassSuccessResponse, stun.MethodBinding, m.TransactionID,
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

	relaySocket := ipv4.NewPacketConn(listener)
	err = relaySocket.SetControlMessage(ipv4.FlagDst, true)
	if err != nil {
		return 0, err
	}

	addr, err := stun.NewTransportAddr(listener.LocalAddr())
	go packetHandler(relaySocket, remoteKey)
	return addr.Port, err
}
