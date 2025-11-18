# custom-logger

`custom-logger` is an example demonstrating how to override the default logging behavior of the [Pion WebRTC](https://github.com/pion/webrtc) stack.  
By default, Pion logs everything to `stdout`.  
This example shows how to inject a **custom `LoggerFactory`** to handle logs from every subsystem (ICE, DTLS, SCTP, DataChannel...).

---

##  Features

- Creates a **custom logger** that implements `logging.LeveledLogger`.
- Initializes two peer connections (`offerer` and `answerer`) locally.
- Establishes a WebRTC connection between them.
- Logs events from:
    - `ICE` candidate gathering
    - `DTLS` handshake
    - `SCTP` and `DataChannel` setup
- Prints logs with clear prefixes like `customLogger Debug:`.

Ideal for:
- Integrate with external monitoring systems
- Store logs to files or databases
- Debug complex WebRTC flows in a structured way

---

##  How to run

### 1. Install the example

```
go install github.com/pion/webrtc/v4/examples/custom-logger@latest
```
Make sure  ```$(go env GOPATH)/bin ```  is in your ```PATH```.

You can add it to your PATH like this (zsh):

```
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc
source ~/.zshrc
```
### 2.Run
`custom-logger` or  `go run main.go`

##  Example output 

```
Creating logger for ice
Creating logger for dtls
Peer Connection State has changed: connected (answerer)
Peer Connection State has changed: connected (offerer)
customLogger Debug: Adding a new peer-reflexive candidate: 10.8.21.1:51196
```


You should see messages from our customLogger, as two PeerConnections start a session
