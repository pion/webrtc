# save-to-disk-mpegts

save-to-disk-mpegts demonstrates how to record an H.264 WebRTC video track into an MPEG-TS file using `pion/format`.

The MPEG-TS package currently supports video only, so this example does not request or record audio.

## Instructions

### Run the example

From this directory, run:

```sh
go run .
```

To select a different output file or listen address:

```sh
go run . -output recording.ts -addr localhost:8081
```

### Record video

Open <http://localhost:8080>, allow camera access, and click **Start recording**. Record for 10–15 seconds, then click **Stop recording**.

The example writes `output.ts` in this directory. The Stop button waits for the MPEG-TS writer to finish; confirm that the page says `Recording saved` and the terminal says `Finished writing output.ts`.

### Test the recording

Run the playback example with the recorded file:

```sh
cd ../play-from-disk-mpegts
go run . -input ../save-to-disk-mpegts/output.ts -addr localhost:8081
```

Then open <http://localhost:8081> and click **Start playback**.
