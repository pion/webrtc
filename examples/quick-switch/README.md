# quick-switch
quick-switch demonstrates how to quickly switch between multiple videos using WebRTC.
Similiar to how sites like TikTok quickly swipe between videos


<video src="https://github.com/user-attachments/assets/07f15044-3aeb-44e2-a866-14bd4258aca0" autoplay loop muted> </video>

The logic in the frontend is purposefully kept as simple as possible. Making it easy to use this on any platform that supports WebRTC.

In the `main.go` we have one video track, and we switch which video file is written to it.

## Instructions

### Download quick-switch
This example requires you to clone the repo since it is serving static HTML.

```
git clone https://github.com/pion/webrtc.git
cd webrtc/examples/quick-switch
```

### Create your videos
This example expects AV1 inside a ivf container. I used the follow to encode for my demo.

```
ffmpeg -y \
  -i $YOUR_INPUT_VIDEO \
  -c:v libaom-av1 \
  -usage realtime \
  -lag-in-frames 0 \
  -crf 30 \
  -b:v 0 \
  -g 15 \
  -keyint_min 15 \
  -sc_threshold 0 \
  -pix_fmt yuv420p \
  -f ivf \
  output.ivf
```


### Run quick-switch
Execute `go run *.go`

### Open the Web UI
Open [http://localhost:8080](http://localhost:8080). This will automatically start a PeerConnection.

Press 'Next Video' and have fun! If you have ideas on how to make it better we would love to hear.
