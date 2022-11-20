#!/usr/bin/env bash
/etc/init.d/ssh start
go install ./examples/data-channels
while true
do
    sleep 10
done
