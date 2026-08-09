# save-to-disk-mpegts

save-to-disk-mpegts is a simple example that receives H.264 video from a browser over WebRTC and writes it to an MPEG-TS file using `pion/format`.

It demonstrates the MPEG-TS writer. This example records video only.

## Run

```sh
cd examples/save-to-disk-mpegts
go run .
```

Open <http://localhost:8080>, allow camera access, and click **Start recording**. Click **Stop recording** when finished. The recording is saved as `output.ts` in this directory.
