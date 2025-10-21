# WebRTC DataChannel Example in Go

This is a minimal example of a **WebRTC DataChannel** using **Go (Pion)** as the signaling server.

## Features

- Go server for signaling
- Browser-based DataChannel
- ICE candidate exchange
- Real-time messaging between browser and Go server

## Usage

1. Run the server:

```
go run main.go
```

2. Open browser at http://localhost:8080

3. Send messages via DataChannel and see them in terminal & browser logs.

