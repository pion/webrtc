# play-from-disk-mpegts

play-from-disk-mpegts demonstrates how to read H.264 or H.265 video from an MPEG-TS file and send it to a browser using Pion WebRTC.

The MPEG-TS package currently supports video only. Audio streams in the input are not sent.

## Instructions

### Create an MPEG-TS file

Place a WebRTC-compatible H.264 file named `output.ts` in this directory. You can create one with FFmpeg:

```sh
ffmpeg -i "$INPUT_FILE" -map 0:v:0 -vf 'scale=1280:-2' \
  -c:v libx264 -preset veryfast -crf 23 \
  -profile:v baseline -level:v 3.1 -pix_fmt yuv420p \
  -bf 0 -g 48 -keyint_min 48 -sc_threshold 0 \
  -an -f mpegts output.ts
```

H.265 files can also be read, but H.265 WebRTC support varies between browsers.

### Run the example

From this directory, run:

```sh
go run .
```

To use a different MPEG-TS file or listen address:

```sh
go run . -input video.ts -addr localhost:8081
```

### Open the browser page

Open <http://localhost:8080> and click **Start playback**.

The Go process serves the page and handles WebRTC signaling. No separate examples server or manual Session Description exchange is required.
