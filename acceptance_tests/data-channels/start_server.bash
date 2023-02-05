GO=/usr/local/go/bin/go
cd "/go/src/github.com/pion/webrtc/examples/data-channels"
TMP=`mktemp`
$GO build -buildvcs=false -o $HOME/datachannels . > $TMP 2>&1

if [ $? -eq 0 ]; then
    echo $1 | $HOME/datachannels
else
    tail -5 $TMP | tr '\n' ':'
fi
rm $TMP
