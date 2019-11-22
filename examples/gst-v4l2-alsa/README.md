# gst-v4l2-alsa
Use GStreamer cli tool handle v4l2 camera device raw video to VP8 

Use GStreamer cli tool handle alsa audio device to Opus.

Use Pion WebRTC connect the video and audio to browser.

Done.

This example base on [play-from-disk](../play-from-disk), only test on Linux, Mac and Windows might not works.

### How to get alsa device
```bash
$ arecord -l
**** List of CAPTURE Hardware Devices ****
card 0: PCH [HDA Intel PCH], device 0: ALC887-VD Analog [ALC887-VD Analog]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
card 0: PCH [HDA Intel PCH], device 2: ALC887-VD Alt Analog [ALC887-VD Alt Analog]
  Subdevices: 1/1
  Subdevice #0: subdevice #0
card 2: Pro [ASTRA Pro], device 0: USB Audio [USB Audio]
  Subdevices: 0/1
  Subdevice #0: subdevice #0
```
```
main.go 

exec.Command(
    "gst-launch-1.0",
    "alsasrc", "device=hw:2",            // HERE, use card 2: Pro [ASTRA Pro] ...
```

### Test your alsa device works
just use `arecord | aplay` make a loop to test
```bash
$ arecord -D hw:2 -f S16_LE -c2 -r 48000 | aplay 
Recording WAVE 'stdin' : Signed 16 bit Little Endian, Rate 48000 Hz, Stereo
Warning: rate is not accurate (requested = 48000Hz, got = 16000Hz)              // HERE
         please, try the plug plugin


$ arecord -D hw:2 -f S16_LE -c2 -r 16000 | aplay 
Recording WAVE 'stdin' : Signed 16 bit Little Endian, Rate 16000 Hz, Stereo
Playing WAVE 'stdin' : Signed 16 bit Little Endian, Rate 16000 Hz, Stereo
```
```
main.go

"alsasrc", "device=hw:2",
"!", "audio/x-raw,format=S16LE,rate=16000,channels=2",      // HERE, use device config
"!", "audioresample",                                       // HERE, resample to 48000
"!", "audio/x-raw,format=S16LE,rate=48000,channels=2",
```

### Linux
Run `echo $BROWSER_SDP | go run .`

### Example page
[jsfiddle.net](https://jsfiddle.net/z7ms3u5r/), this is [play-from-disk](../play-from-disk/README.md) example pages, it works.

#### Input SessionDescription into your browser
Copy the text that just emitted and copy into second text area

#### Hit 'Start Session' in jsfiddle, enjoy your video!
A video and Audio should start playing in your browser above the input boxes. 

Have fun.
