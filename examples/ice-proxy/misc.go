package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"

	"github.com/pion/turn/v4"
	"golang.org/x/net/proxy"
)

var _ proxy.Dialer = &proxyDialer{}

type proxyDialer struct {
	proxyAddr string
}

func newProxyDialer(u *url.URL) (proxy.Dialer, error) {
	if u.Scheme != "http" {
		return nil, fmt.Errorf("unsupported scheme in proxy URL: %v", u.Scheme)
	}

	return &proxyDialer{
		proxyAddr: u.Host,
	}, nil
}

func (d *proxyDialer) Dial(network, addr string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported network type: %v", network)
	}

	conn, err := net.Dial(network, d.proxyAddr)
	if err != nil {
		return nil, fmt.Errorf("connect to proxy %q: %w", d.proxyAddr, err)
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
		return nil, fmt.Errorf("write request: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("connect status: %d %q, read response: %w", resp.StatusCode, resp.Status, err)
		}
		return nil, fmt.Errorf("connect status: %d %q, response body: %q", resp.StatusCode, resp.Status, string(body))
	}

	return conn, nil
}

func newHTTPProxy() (*url.URL, net.Listener, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen: %w", err)
	}

	go func() {
		defer listener.Close()
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
		Host:   fmt.Sprintf("127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port),
	}, listener, nil
}

func proxyHandleConn(clientConn net.Conn) {
	// Read the request from the client
	req, err := http.ReadRequest(bufio.NewReader(clientConn))
	if err != nil {
		log.Printf("Failed to read proxy init request: %v", err)
		return
	}

	if req.Method != http.MethodConnect {
		log.Printf("Proxy init request is not a CONNECT request: %v", req.Method)
		return
	}

	// Establish a connection to the target server
	targetConn, err := net.Dial("tcp", req.URL.Host)
	if err != nil {
		log.Printf("Failed to connect to target (%q) from proxy: %v", req.URL.Host, err)
		return
	}

	// Answer to the client with a 200 OK response
	if _, err := clientConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
		log.Printf("Failed to write response to the proxy client: %v", err)
		return
	}

	// Copy data between client and target
	go io.Copy(clientConn, targetConn)
	go io.Copy(targetConn, clientConn)
}

func newTURNServer() (*turn.Server, error) {
	tcpListener, err := net.Listen("tcp4", "127.0.0.1:17342")
	if err != nil {
		return nil, err
	}

	server, err := turn.NewServer(turn.ServerConfig{
		AuthHandler: func(username, realm string, addr net.Addr) ([]byte, bool) {
			log.Printf("Request to TURN from %q", addr.String())
			return turn.GenerateAuthKey("turn_username", realm, "turn_password"), true
		},
		ListenerConfigs: []turn.ListenerConfig{
			{
				Listener: tcpListener,
				RelayAddressGenerator: &turn.RelayAddressGeneratorStatic{
					RelayAddress: net.ParseIP("127.0.0.1"),
					Address:      "127.0.0.1",
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	return server, nil
}
