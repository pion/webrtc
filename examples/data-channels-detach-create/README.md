# data-channels-detach-create
data-channels-detach is an example that shows how you can detach a data channel. This allows direct access the underlying [pion/datachannel](https://github.com/pion/datachannel). This allows you to interact with the data channel using a more idiomatic API based on the `io.ReadWriteCloser` interface.

The example is meant to be used with data-channels-detach. This demonstrates two Go Pion processes communicating directly.

## Run data-channels-detach-create and make an offer to data-channels-detach via stdin
```
go run data-channels-detach-create/*.go | go run data-channels-detach/*.go
```

## post the answer from data-channels-detach back to data-channels-detach-create
You will see a base64 SDP printed to your console. You now need to communicate this back to `data-channels-detach-create` this can be done via a HTTP endpoint

`curl localhost:8080/sdp -d "BASE_64_SDP"`

## Output

On sucess you will get output like the following

```
Peer Connection State has changed: connecting
(Long base64 SDP that you should POST)
Peer Connection State has changed: connected
New DataChannel  1374394845054
Data channel ''-'1374394845054' open.
Message from DataChannel: kvmWkjYodyQcIlv
Sending aMDnwlTfDYnfoUy
Sending htqQtnbvygZKlmy
Message from DataChannel: CMjZiNtsmIBpCaN
```
