# save-to-disk-mpegts

This example records an H.264 WebRTC video track into an MPEG-TS file using the writer from `pion/format`.

## Run

1. Start the local examples server from the `examples` directory:

   ```sh
   go run examples.go --address localhost:8080
   ```

2. Open <http://localhost:8080/example/js/save-to-disk-mpegts/>.
3. Copy the browser Session Description.
4. In another terminal, run the program from its example directory:

   ```sh
   cd /Users/gokuljs/work-docs/webrtc/examples/save-to-disk-mpegts
   echo "$BROWSER_SDP" | go run .
   ```

5. Paste the Session Description printed by the Go program into the browser page and start the session.
6. Record for a few seconds, then close the browser tab.

The example writes `output.ts` inside `examples/save-to-disk-mpegts`. To test the complete round trip, copy that file to `examples/play-from-disk-mpegts/output.ts` and run the playback example.
