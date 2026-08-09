# play-from-disk-mpegts

This example reads H.264 or H.265 access units from an MPEG-TS file using `pion/format` and sends them to a browser over WebRTC.

## Video location

Place the video at `output.ts` in the directory from which you run the example. When running from the repository root, that means:

```text
/Users/gokuljs/work-docs/webrtc/output.ts
```

## Run

1. Open the existing [play-from-disk browser page](https://jsfiddle.net/8kup9mvn/).
2. Copy the browser Session Description.
3. From the repository root, run:

   ```sh
   echo "$BROWSER_SDP" | go run ./examples/play-from-disk-mpegts
   ```

4. Paste the Session Description printed by the Go program into the browser page and start the session.

The example discovers whether the first supported video track is H.264 or H.265. Browser support for H.265 varies; H.264 is the recommended format for this test.
