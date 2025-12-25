# play-from-device

play-from-device is a modified version of [examples/play-from-disk-renegotiation](/examples/play-from-disk-renegotiation) that captures audio and video from local devices (camera and microphone) and streams them via WebRTC.

This example is designed and tested on **Ubuntu 22.04**.

## Features

- **Audio Support**: Opus codec encoding for low-latency audio transmission
- **Video Support**: H.264 codec encoding for efficient video transmission
- **Multiple Backends**: Supports both FFmpeg and GStreamer for media processing
- **Dynamic Track Management**: Add/remove audio and video tracks during runtime
- **Low Latency**: Optimized for real-time streaming with minimal delay

## Requirements

### System Requirements
- Ubuntu 22.04
- Go (Golang) installed
- Either FFmpeg or GStreamer installed

### Hardware Requirements
- Camera device (typically `/dev/video0`)
- Audio capture device (configured as `hw:2,0` in the code)

### Software Installation

#### FFmpeg (Optional)
```bash
sudo apt update
sudo apt install ffmpeg
```

#### GStreamer (Default)
```bash
sudo apt update
sudo apt install gstreamer1.0-tools gstreamer1.0-plugins-good gstreamer1.0-plugins-bad gstreamer1.0-plugins-ugly gstreamer1.0-libav
```

## Configuration

### Audio Device Configuration
The default audio device is set to `hw:2,0`. To check available audio devices:
```bash
arecord -l
```

Update the device in the code if your audio device has a different number.

### Video Device Configuration
The default video device is `/dev/video0`. To check available video devices:
```bash
ls /dev/video*
v4l2-ctl --list-devices
```
