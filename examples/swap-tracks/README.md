# swap-tracks

swap-tracks demonstrates how to swap multiple incoming tracks on a single outgoing track.

## [architecture](https://viewer.diagrams.net/?tags=%7B%7D&highlight=FFFFFF&edit=_blank&layers=1&nav=1&title=drawio#Uhttps%3A%2F%2Fraw.githubusercontent.com%2Fmohammadne%2Fwebrtc-pion%2Fmaster%2Fexamples%2Fswap-tracks%2Fdrawio)

## Instructions

### Download swap-tracks

```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/swap-tracks
```

### Open swap-tracks example page

[jsfiddle.net](https://jsfiddle.net/dzc17fga/) you should see two text-areas and a 'Start Session' button.

### Run swap-tracks, with your browsers SessionDescription as stdin

In the jsfiddle the top textarea is your browser, copy that and:

#### Linux/macOS

Run `echo $BROWSER_SDP | swap-tracks`

#### Windows

1. Paste the SessionDescription into a file.
1. Run `swap-tracks < my_file`

### Input swap-tracks's SessionDescription into your browser

Copy the text that `swap-tracks` just emitted and copy into second text area

### Hit 'Start Session' in jsfiddle, enjoy your video

Your browser should send streams to Pion, and then a stream will be relayed back, changing every 5 seconds.

Congrats, you have used Pion WebRTC! Now start building something cool
