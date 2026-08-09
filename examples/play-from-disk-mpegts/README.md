# play-from-disk-mpegts

This example reads H.264 or H.265 access units from an MPEG-TS file using `pion/format` and sends them to a browser over WebRTC.

## Video location

Place the video inside this example directory with the exact name `output.ts`:

```text
/Users/gokuljs/work-docs/webrtc/examples/play-from-disk-mpegts/output.ts
```

## Run

1. Start the local examples server from the `examples` directory:

   ```sh
   go run examples.go --address localhost:8080
   ```

2. Open <http://localhost:8080/example/js/play-from-disk-mpegts/>.
3. Copy the browser Session Description.
4. In another terminal, run the program from its example directory:

   ```sh
   cd /Users/gokuljs/work-docs/webrtc/examples/play-from-disk-mpegts
   echo "$BROWSER_SDP" | go run .
   ```

5. Paste the Session Description printed by the Go program into the browser page and start the session.

The example discovers whether the first supported video track is H.264 or H.265. Browser support for H.265 varies; H.264 is the recommended format for this test.
