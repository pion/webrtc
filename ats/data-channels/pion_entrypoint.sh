#!/usr/bin/env bash
# The pion entry point for the data channel example
# First install the command, which can take a while, and only then open ssh server
go install ./examples/data-channels
/etc/init.d/ssh start
while true
do
    sleep 10
done
