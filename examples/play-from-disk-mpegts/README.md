# play-from-disk-mpegts

play-from-disk-mpegts is a simple example that reads H.264 or H.265 video from an MPEG-TS file and sends it to a browser over WebRTC.

It demonstrates the MPEG-TS reader. This example plays video only.

## Run

Place an MPEG-TS file named `output.ts` in this directory, then run:

```sh
cd examples/play-from-disk-mpegts
go run .
```

To play the file created by `save-to-disk-mpegts`, run:

```sh
cd examples/play-from-disk-mpegts
go run . -input ../save-to-disk-mpegts/output.ts
```

Open <http://localhost:8080> and click **Start playback**.
