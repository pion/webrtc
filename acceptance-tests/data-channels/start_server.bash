#!/bin/bash
GO=/usr/local/go/bin/go
cd "/go/src/github.com/pion/webrtc/examples/data-channels"
TMP=`mktemp`
$GO build -buildvcs=false -o $HOME/datachannels . > $TMP 2>&1

if [ $? -eq 0 ]; then
    echo $1 | $HOME/datachannels
else
    # on error send the last 5 lines of output as a single line
    # so it's displayed in the browser
    tail -5 $TMP | tr '\n' ':'
fi
rm $TMP
