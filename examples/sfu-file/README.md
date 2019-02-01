# sfu-file

This is a solution for the 4096 chars problem when use sfu.

## Instructions
### Download sfu-file
```
go get github.com/pions/webrtc/examples/sfu-file
```

### Open example page
[jsfiddle.net](https://jsfiddle.net/5cwx0rns/11/) You should see two buttons 'Publish a Broadcast' and 'Join a Broadcast'


### Start a publisher
Click `Publish a Broadcast` and save the text to sdp.txt. 

### Join the broadcast
Click `Join a Broadcast` and save the text to sdp1.txt. 


### Run program
run `main.go`

It will respond with an offer, paste it to the publish page second input field. Then press `Start Session`

Then the program will respond with another offer,  paste it to the join  page second input field. Then press `Start Session`. Now you can see the publisher!

Congrats, you have used pion-WebRTC! Now start building something cool
