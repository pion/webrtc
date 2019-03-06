# save-to-disk
This is a simple application that captures video and audio from your computer and save it to disk using pion-WebRTC. This example involves two peers, your browser and `save-to-disk` application. Peers have to exchange Session Descriptions in order to establish a connection.

## Instructions
There are three key parts in this example:
* The first part is getting Browsers Session Description
* The second part is getting `save-to-disk` Session Description
* The third part is starting the session and sending video and audio streams from browser to `save-to-disk` application

### Part one - getting browser's Session Description
Browser code lives inside `jsfiddle` directory but you can run it on this [jsfiddle.net](https://jsfiddle.net/dyj8qpek/19/) link.

Browser will prompt you that the page needs permissions to acces your video and audio device. Once you grant permissions, you should be able to see video from your webcam on the jsfiddle page.

The page has two input fields and a video element. The first input field `Browser base64 Session Description` should be prepopulated. The second input field `Golang base64 Session Description` will be empty.

Copy contents of `Browser base64 Session Description` input field.

### Part two - getting save-to-disk Session Description
Make sure you have `save-to-disk` example on your machine
```
go get github.com/pions/webrtc/examples/save-to-disk
```

Change directory to be `save-to-disk` directory
```
cd $GOPATH/src/github.com/pions/webrtc/examples/save-to-disk
```

When running this example, paste Browser's Session Description as the first argument:
```
go run main.go PASTE_BROWSER_SESSION_DESCRIPTION_HERE
```
The application will print out `save-to-disk` Session Description. Copy it and paste it in the jsfiddle `Golang base64 Session Description` input field.

### Part three - starting the session
Now both peers know about each other and they are ready to connect. Go to the jsfiddle page and hit 'Start Session'.
Your video and audio will be sent from the browser to `save-to-disk` application. When you're done, stop `save-to-disk` and in the directory you ran it you should have two files `output.ivf` and `output.opus`

Congrats, you have used pion-WebRTC! Now start building something cool
