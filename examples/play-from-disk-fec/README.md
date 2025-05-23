# play-from-disk-fec
play-from-disk-fec demonstrates how to use forward error correction (FlexFEC-03) while sending video to your Chrome-based browser from files saved to disk. The example is designed to drop 40% of the media packets, but browser will recover them using the FEC packets and the delivered packets.

## Instructions
### Create IVF named `output.ivf` that contains a VP8/VP9/AV1 track
```
ffmpeg -i $INPUT_FILE -g 30 -b:v 2M output.ivf
```

**Note**: In the `ffmpeg` command which produces the .ivf file, the argument `-b:v 2M` specifies the video bitrate to be 2 megabits per second. We provide this default value to produce decent video quality, but if you experience problems with this configuration (such as dropped frames etc.), you can decrease this. See the [ffmpeg documentation](https://ffmpeg.org/ffmpeg.html#Options) for more information on the format of the value.

### Download play-from-disk-fec

```
go install github.com/pion/webrtc/v4/examples/play-from-disk-fec@latest
```

### Open play-from-disk-fec example page
Open [jsfiddle.net](https://jsfiddle.net/hgzwr9cm/) in your browser. You should see two text-areas and buttons for the offer-answer exchange.

### Run play-from-disk-fec to generate an offer
The `output.ivf` you created should be in the same directory as `play-from-disk-fec`.

When you run play-from-disk-fec, it will generate an offer in base64 format and print it to stdout.

### Input play-from-disk-fec's offer into your browser
Copy the base64 offer that `play-from-disk-fec` just emitted and paste it into the first text area in the jsfiddle (labeled "Remote Session Description")

### Hit 'Start Session' in jsfiddle to generate an answer
Click the 'Start Session' button. This will process the offer and generate an answer, which will appear in the second text area.

### Save the browser's answer to a file
Copy the base64-encoded answer from the second text area (labeled "Browser Session Description") and save it to a file named `answer.txt` in the same directory where you're running `play-from-disk-fec`.

### Press Enter to continue
Once you've saved the answer to `answer.txt`, go back to the terminal where `play-from-disk-fec` is running and press Enter. The program will read the answer file and establish the connection.

### Enjoy your video!
A video should start playing in your browser above the input boxes. `play-from-disk-fec` will exit when the file reaches the end

You can watch the stats about transmitted/dropped media & FEC packets in the stdout.

Congrats, you have used Pion WebRTC! Now start building something cool
