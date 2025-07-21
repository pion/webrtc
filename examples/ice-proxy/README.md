# ICE Proxy
`ice-proxy` demonstrates Pion WebRTC's capabilities for utilizing a proxy in WebRTC connections.

This proxy functionality is particularly useful when direct peer-to-peer communication is restricted, such as in environments with strict firewalls. It primarily leverages TURN (Traversal Using Relays around NAT) with TCP connections to enable communication with the outside world.

## Instructions

### Download ice-proxy
The example is self-contained and requires no input.

```bash
go install github.com/pion/webrtc/v4/examples/ice-proxy@latest
```

### Run ice-proxy
```bash
ice-proxy
```

Upon execution, four distinct entities will be launched:
* `TURN Server`: This server facilitates relaying media traffic when direct communication between agents is not possible, simulating a scenario where peers are behind restrictive NATs.
* `Proxy HTTP Server`: A straightforward HTTP proxy designed to forward all TCP traffic to a specified target.
* `Offering Agent`: In a typical WebRTC setup, this would be a web browser. In this example, it's a simplified Pion client that initiates the WebRTC connection. This agent attempts direct communication with the answering agent.
* `Answering Agent`: This typically represents a web server. In this demonstration, it's configured to use the TURN server, simulating a scenario where the agent is not directly reachable. This agent exclusively uses a relay connection via the TURN server, with a proxy acting as an intermediary between the agent and the TURN server.


