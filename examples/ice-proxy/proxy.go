// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

//go:build !js
// +build !js

package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/proxy"
)

var _ proxy.Dialer = &proxyDialer{}

type proxyDialer struct {
	proxyAddr string
}

func newProxyDialer(u *url.URL) proxy.Dialer {
	if u.Scheme != "http" {
		panic("unsupported proxy scheme")
	}

	return &proxyDialer{
		proxyAddr: u.Host,
	}
}

func (d *proxyDialer) Dial(network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		panic("unsupported proxy network type")
	}

	conn, err := net.Dial(network, d.proxyAddr) // nolint: noctx
	if err != nil {
		panic(err)
	}

	// Create a CONNECT request to the proxy with target address.
	req := &http.Request{
		Method: http.MethodConnect,
		URL:    &url.URL{Host: addr},
		Header: http.Header{
			"Proxy-Connection": []string{"Keep-Alive"},
		},
	}

	err = req.Write(conn)
	if err != nil {
		panic(err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Printf("close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		panic("unexpected proxy status code: " + resp.Status)
	}

	return conn, nil
}

func newHTTPProxy() *url.URL {
	listener, err := net.Listen("tcp", "localhost:0") // nolint: noctx
	if err != nil {
		panic(err)
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go proxyHandleConn(conn)
		}
	}()

	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("localhost:%d", listener.Addr().(*net.TCPAddr).Port), // nolint:forcetypeassert
	}
}

func proxyHandleConn(clientConn net.Conn) {
	// Read the request from the client
	req, err := http.ReadRequest(bufio.NewReader(clientConn))
	if err != nil {
		panic(err)
	}

	if req.Method != http.MethodConnect {
		panic("unexpected request method: " + req.Method)
	}

	// Establish a connection to the target server
	targetConn, err := net.Dial("tcp", req.URL.Host) // nolint: noctx
	if err != nil {
		panic(err)
	}

	// Answer to the client with a 200 OK response
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
		panic(err)
	}

	// Copy data between client and target
	go io.Copy(clientConn, targetConn) // nolint: errcheck
	go io.Copy(targetConn, clientConn) // nolint: errcheck
}
