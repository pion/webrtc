# SCTP Interleaving Inspector

This browser example uses `pion/ice`, `pion/dtls`, `pion/sctp`, and
`pion/datachannel` directly so it can intercept raw SCTP messages and parses them.

```sh
go run .
```

Open <http://localhost:8080> and click **Connect**. The page creates the
browser offer automatically, connects to the Pion stack, and reports whether
INIT/INIT-ACK advertised I-DATA chunk type 64, whether I-DATA was actually
observed, and whether legacy DATA chunks were seen.

As I wrote this, firefox should have I-DATA enabled by default, for chrome, you'll need to run chrome with this flag:

```
[chrome] -user-data-dir=/tmp/chrome-sctp-test --force-fieldtrials="WebRTC-DataChannel-Dcsctp/Enabled/WebRTC-DataChannelMessageInterleaving/Enabled/" http://localhost:8080
```
