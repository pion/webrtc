package main

import (
	"time"

	"github.com/pion/logging"
	"github.com/pion/transport/vnet"
	"github.com/pion/webrtc/v2"
)

func main() {
	wan, err := vnet.NewRouter(&vnet.RouterConfig{
		CIDR:          "1.2.3.0/24",
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	})
	panicIfError(err)

	offerVNet := vnet.NewNet(&vnet.NetConfig{})
	panicIfError(wan.AddNet(offerVNet))

	offerSettingEngine := webrtc.SettingEngine{}
	offerSettingEngine.SetVNet(offerVNet)
	offerAPI := webrtc.NewAPI(webrtc.WithSettingEngine(offerSettingEngine))

	answerVNet := vnet.NewNet(&vnet.NetConfig{})
	panicIfError(wan.AddNet(answerVNet))

	answerSettingEngine := webrtc.SettingEngine{}
	answerSettingEngine.SetVNet(answerVNet)
	answerAPI := webrtc.NewAPI(webrtc.WithSettingEngine(answerSettingEngine))

	panicIfError(wan.Start())

	offerPeerConnection, err := offerAPI.NewPeerConnection(webrtc.Configuration{})
	panicIfError(err)

	answerPeerConnection, err := answerAPI.NewPeerConnection(webrtc.Configuration{})
	panicIfError(err)

	offerDataChannel, err := offerPeerConnection.CreateDataChannel("label", nil)
	panicIfError(err)

	msgSendLoop := func(dc *webrtc.DataChannel) {
		for {
			time.Sleep(500 * time.Millisecond)
			panicIfError(dc.SendText("My DataChannel Message"))
		}

	}

	offerDataChannel.OnOpen(func() {
		msgSendLoop(offerDataChannel)
	})

	answerPeerConnection.OnDataChannel(func(answerDataChannel *webrtc.DataChannel) {
		answerDataChannel.OnOpen(func() {
			msgSendLoop(answerDataChannel)
		})
	})

	offer, err := offerPeerConnection.CreateOffer(nil)
	panicIfError(err)
	panicIfError(offerPeerConnection.SetLocalDescription(offer))
	panicIfError(answerPeerConnection.SetRemoteDescription(offer))

	answer, err := answerPeerConnection.CreateAnswer(nil)
	panicIfError(err)
	panicIfError(answerPeerConnection.SetLocalDescription(answer))
	panicIfError(offerPeerConnection.SetRemoteDescription(answer))

	select {}
}

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}
