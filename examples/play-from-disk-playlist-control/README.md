# ogg-playlist-sctp
Streams Opus pages from multi or single track Ogg containers, exposes the playlist over an SCTP DataChannel, and lets the browser hop between tracks while showing artist/title metadata parsed from OpusTags.

## What this showcases
- Reads multi-stream Ogg containers with `oggreader` and keeps per-serial playback state.
- Publishes playlist + now-playing metadata (artist/title/vendor/comments) over a DataChannel.
- Browser can send `next`, `prev`, or a 1-based track number to jump around.
- Audio is sent as an Opus `TrackLocalStaticSample` over RTP, metadata/control ride over SCTP.

## Prepare a demo playlist
The example looks for `playlist.ogg` in the working directory.
You can provide your own `playlist.ogg` or generate it by running one of the following ffmpeg commands:

**Fake two-track Ogg with metadata (artist/title per stream)**
```sh
ffmpeg \
  -f lavfi -t 8 -i "sine=frequency=330" \
  -f lavfi -t 8 -i "sine=frequency=660" \
  -map 0:a -map 1:a \
  -c:a libopus -page_duration 20000 \
  -metadata:s:a:0 artist="Pion Artist" -metadata:s:a:0 title="Fake Intro" \
  -metadata:s:a:1 artist="Open-Source Friend" -metadata:s:a:1 title="Fake Outro" \
  playlist.ogg
```

**Single-track fallback with tags**
```sh
ffmpeg -f lavfi -t 10 -i "sine=frequency=480" \
  -c:a libopus -page_duration 20000 \
  -metadata artist="Solo Bot" -metadata title="One Track Demo" \
  playlist.ogg
```

## Run it
1. Build the binary:
   ```sh
   go install github.com/pion/webrtc/v4/examples/play-from-disk-playlist-control@latest
   ```
2. Run it from the directory containing `playlist.ogg` (override port with `-addr` if you like):
   ```sh
   play-from-disk-playlist-control
   # or
   play-from-disk-playlist-control -addr :8080
   ```
3. Open the hosted UI in your browser and press **Start Session**:
   ```
   http://localhost:8080
   ```
   Signaling is WHEP-style: the browser POSTs plain SDP to `/whep` and the server responds with the answer SDP. Use the buttons or type `next` / `prev` / a track number to switch tracks. Playlist metadata and now-playing updates arrive over the DataChannel; Opus audio flows on the media track.
