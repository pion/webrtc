# read-rtx
read-rtx is a simple application that shows how to record your webcam/microphone using Pion WebRTC and read packets from streams of rtp and rtx

## Instructions
### Download read-rtx
```
export GO111MODULE=on
go get github.com/pion/webrtc/v3/examples/read-rtx
```

### Open read-rtx example page
[jsfiddle.net](https://jsfiddle.net/s179hacu/) you should see your Webcam, two text-areas and two buttons: `Copy browser SDP to clipboard`, `Start Session`.

### Run read-rtx, with your browsers SessionDescription as stdin
In the jsfiddle the top textarea is your browser's Session Description. Press `Copy browser SDP to clipboard` or copy the base64 string manually.
We will use this value in the next step.

#### Linux/macOS
Run `echo $BROWSER_SDP | read-rtx` 

or

Run `cd examples/read-rtx` and then
`echo $BROWSER_SDP | go run .`

#### Windows
1. Paste the SessionDescription into a file.
1. Run `read-rtx < my_file`

### Input read-rtx's SessionDescription into your browser
Copy the text that `read-rtx` just emitted and copy into second text area

### Hit 'Start Session'
You will see output like below:
```bash
Connection State has changed connected 
Ctrl+C the remote client to stop the demo
Got Audio track hasRTX: false
Got Video track hasRTX: true
Got RTX padding packets. rtx sn: 24254
Got RTX padding packets. rtx sn: 24255
Send Nack sequence:17721
Got RTX Packet. osn: 17721 , rtx sn: 24256
Send Nack sequence:17791
Got RTX Packet. osn: 17791 , rtx sn: 24257
Send Nack sequence:17857
Got RTX Packet. osn: 17857 , rtx sn: 24258
Send Nack sequence:17929
Got RTX Packet. osn: 17929 , rtx sn: 24259
Send Nack sequence:17999
Got RTX Packet. osn: 17999 , rtx sn: 24260
Send Nack sequence:18063
Got RTX Packet. osn: 18063 , rtx sn: 24261
Send Nack sequence:18123
Got RTX Packet. osn: 18123 , rtx sn: 24262
Got RTX padding packets. rtx sn: 24263
Got RTX Packet. osn: 18185 , rtx sn: 24264
Got RTX padding packets. rtx sn: 24265
Got RTX Packet. osn: 18186 , rtx sn: 24266
Got RTX padding packets. rtx sn: 24267
Got RTX Packet. osn: 18184 , rtx sn: 24268
Got RTX Packet. osn: 18183 , rtx sn: 24269
Got RTX Packet. osn: 18182 , rtx sn: 24270
Got RTX Packet. osn: 18181 , rtx sn: 24271
Got RTX Packet. osn: 18180 , rtx sn: 24272
Got RTX Packet. osn: 18179 , rtx sn: 24273
Got RTX Packet. osn: 18178 , rtx sn: 24274
Send Nack sequence:18190
Got RTX Packet. osn: 18190 , rtx sn: 24275
Send Nack sequence:18303
Got RTX Packet. osn: 18303 , rtx sn: 24276
Send Nack sequence:18434
Got RTX Packet. osn: 18434 , rtx sn: 24277
Send Nack sequence:18608
Got RTX Packet. osn: 18608 , rtx sn: 24278

```