GO=/usr/local/go/bin/go
cd "/go/src/github.com/pion/webrtc/examples/data-channels"
TMP=`mktemp`
$GO build . > $TMP 2>&1

if [ $? -eq 0 ]; then
    echo $1 | ./data-channels
else
    cat $TMP
fi
rm $TMP
