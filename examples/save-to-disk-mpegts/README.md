# save-to-disk-mpegts

This example records an H.264 WebRTC video track into an MPEG-TS file using the writer from `pion/format`.

## Run

1. Open the existing [save-to-disk browser page](https://jsfiddle.net/2nwt1vjq/).
2. Copy the browser Session Description.
3. From the repository root, run:

   ```sh
   echo "$BROWSER_SDP" | go run ./examples/save-to-disk-mpegts
   ```

4. Paste the Session Description printed by the Go program into the browser page and start the session.
5. Record for a few seconds, then close the browser tab.

The example writes `output.ts` in the directory from which you run the command. Use that file with `play-from-disk-mpegts` to test the complete writer/reader round trip.
