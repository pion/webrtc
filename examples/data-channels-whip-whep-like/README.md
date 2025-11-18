# whip-whep-like

This example demonstrates a WHIP/WHEP-like implementation using Pion WebRTC with DataChannel support for real-time chat.

**Note:** This is similar to but not exactly WHIP/WHEP, as the official WHIP/WHEP specifications focus on media streaming only and do not include DataChannel support. This example extends the WHIP/WHEP pattern to demonstrate peer-to-peer chat functionality with automatic username assignment and message broadcasting.

Key features:
- **Real-time chat** with WebRTC DataChannels
- **Automatic username generation** - Each user gets a unique random username (e.g., SneakyBear46)
- **Message broadcasting** - All connected users receive messages from everyone else
- **WHIP/WHEP-like signaling** - Simple HTTP-based signaling for easy integration

Further details about WHIP+WHEP and the WebRTC DataChannel implementation are below the instructions.

## Instructions

### Download the example

This example requires you to clone the repo since it is serving static HTML.

```
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/data-channels-whip-whep-like
```

### Run the server
Execute `go run *.go`

### Connect and chat

1. Open [http://localhost:8080](http://localhost:8080) in your browser
2. Click "Publish" or "Subscribe" to establish a DataChannel connection
3. You'll be assigned a random username (e.g., "SneakyBear46")
4. Type a message and click "Send Message" to broadcast to all connected users
5. Open multiple tabs/windows to test multi-user chat

Congrats, you have used Pion WebRTC! Now start building something cool

## Why WHIP/WHEP for signaling?

This example uses a WHIP/WHEP-like signaling approach where an Offer is uploaded via HTTP and the server responds with an Answer. This simple API contract makes it easy to integrate WebRTC into web applications.

**Difference from standard WHIP/WHEP:** The official WHIP/WHEP specifications are designed for media streaming (audio/video) only. This example extends that pattern to include DataChannel support for real-time chat functionality.

## Implementation details

### Username generation
Each connected user is automatically assigned a unique username combining:
- An adjective (e.g., Sneaky, Brave, Quick)
- An animal noun (e.g., Bear, Fox, Eagle)
- A random number (0-999)

Congrats, you have used Pion WebRTC! Now start building something cool

## Why WHIP/WHEP?

WHIP/WHEP mandates that a Offer is uploaded via HTTP. The server responds with a Answer. With this strong API contract WebRTC support can be added to tools like OBS.

For more info on WHIP/WHEP specification, feel free to read some of these great resources:
- https://webrtchacks.com/webrtc-cracks-the-whip-on-obs/
- https://datatracker.ietf.org/doc/draft-ietf-wish-whip/
- https://datatracker.ietf.org/doc/draft-ietf-wish-whep/
- https://bloggeek.me/whip-whep-webrtc-live-streaming
