package stun

import (
	"fmt"
	"net"
	"time"

	"github.com/pions/pkg/stun"
	"github.com/pions/webrtc/pkg/ice"
	"github.com/pkg/errors"
)

// TODO: This file doesn't make sense
// Package ICE should rely on stun, not the other way around.

// AllocateUDP crafts and sends a STUN binding
// On success will return our XORMappedAddress
func AllocateUDP(url *ice.URL) (*net.UDPAddr, *stun.XorAddress, error) {
	// TODO Do we want the timeout to be configurable?
	// proto := url.Proto.String()
	// TODO: Temporary fix for nat traversal issue: Find a permanent solution.
	client, err := stun.NewClient("udp4", fmt.Sprintf("%s:%d", url.Host, url.Port), time.Second*5)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to create STUN client")
	}
	localAddr, ok := client.LocalAddr().(*net.UDPAddr)
	if !ok {
		return nil, nil, errors.Errorf("Failed to cast STUN client to UDPAddr")
	}

	resp, err := client.Request()
	if err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to make STUN request")
	}

	if err = client.Close(); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to close STUN client")
	}

	attr, ok := resp.GetOneAttribute(stun.AttrXORMappedAddress)
	if !ok {
		return nil, nil, errors.Errorf("Got respond from STUN server that did not contain XORAddress")
	}

	var addr stun.XorAddress
	if err = addr.Unpack(resp, attr); err != nil {
		return nil, nil, errors.Wrapf(err, "Failed to unpack STUN XorAddress response")
	}

	return localAddr, &addr, nil
}
