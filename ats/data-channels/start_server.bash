GO=/usr/local/go/bin/go
cd "/go/src/github.com/pion/webrtc/examples/data-channels"
$GO build . > /dev/null 2>&1
echo $1 | ./data-channels
