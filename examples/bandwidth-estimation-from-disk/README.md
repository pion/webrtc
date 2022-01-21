# bandwidth-estimation-from-disk
bandwidth-estimation-from-disk demonstrates how to use Pion's Bandwidth Estimation APIs.

Pion provides multiple Bandwidth Estimators, but they all satisfy one interface. This interface
emits an int for how much bandwidth is available to send. It is then up to the sender to meet that number.

## Instructions
### Create IVF files named `high.ivf` `med.ivf` and `low.ivf`
```
ffmpeg -i $INPUT_FILE -g 30 -b:v .3M  -s 320x240   low.ivf
ffmpeg -i $INPUT_FILE -g 30 -b:v 1M   -s 858x480   med.ivf
ffmpeg -i $INPUT_FILE -g 30 -b:v 2.5M -s 1280x720  high.ivf
```

### Download bandwidth-estimation-from-disk

```
go get github.com/pion/webrtc/v3/examples/bandwidth-estimation-from-disk
```

### Open bandwidth-estimation-from-disk example page
[jsfiddle.net](https://jsfiddle.net/a1cz42op/) you should see two text-areas, 'Start Session' button and 'Copy browser SessionDescription to clipboard'

### Run bandwidth-estimation-from-disk with your browsers Session Description as stdin
The `output.ivf` you created should be in the same directory as `bandwidth-estimation-from-disk`. In the jsfiddle press 'Copy browser Session Description to clipboard' or copy the base64 string manually.

Now use this value you just copied as the input to `bandwidth-estimation-from-disk`

#### Linux/macOS
Run `echo $BROWSER_SDP | bandwidth-estimation-from-disk`
#### Windows
1. Paste the SessionDescription into a file.
1. Run `bandwidth-estimation-from-disk < my_file`

### Input bandwidth-estimation-from-disk's Session Description into your browser
Copy the text that `bandwidth-estimation-from-disk` just emitted and copy into the second text area in the jsfiddle

### Hit 'Start Session' in jsfiddle, enjoy your video!
A video should start playing in your browser above the input boxes. When `bandwidth-estimation-from-disk` switches quality levels it will print the old and new file like so.

```
Switching from low.ivf to med.ivf
Switching from med.ivf to high.ivf
Switching from high.ivf to med.ivf
```


Congrats, you have used Pion WebRTC! Now start building something cool
