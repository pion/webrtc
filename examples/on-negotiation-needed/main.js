var pc = new RTCPeerConnection({iceServers: [{urls: ["stun:stun.l.google.com:19302"]}]})
var channels = []

const MessageTypeICE           = "ice-candidate"
const MessageTypeOffer         = "offer"
const MessageTypeAnswer        = "answer"
// It is only from server
const MessageTypeNewTrack      = "add-track"
const MessageTypeRemoveTrack   = "remove-track"
const MessageTypeAddChannel    = "add-channel"
const MessageTypeRemoveChannel = "remove-channel"

function serverSay() {
	fetch("/server-say")
	.then(res => res.json())
	.then(async msg => {
		console.log("received", msg)
		switch (msg.type) {
			case MessageTypeICE:
				pc.addIceCandidate(msg.candidate)
				.catch(console.error)
				break;
			case MessageTypeOffer:
				try {
					await pc.setRemoteDescription(msg.description)
					var answer = await pc.createAnswer()
					await pc.setLocalDescription(answer)
					clientSay({type: MessageTypeAnswer, description: answer})
				} catch (err) {
					console.error(err)
				}
				break;
			case MessageTypeAnswer:
				pc.setRemoteDescription(msg.description)
				.catch(console.error)
				break;
			default:
				console.log("Unknown message type:", msg.type)
				break;
		}
		serverSay()
	})
}
serverSay()

function clientSay(msg) {
	console.log("sent", msg)
	var json = JSON.stringify(msg)
	fetch("/client-say",
	{
		method: 'post',
		headers: {'content-type': 'application/json'},
		body: json,
	})
}

pc.onnegotiationneeded = async e => {
	// Dont use this, see
	// https://developer.mozilla.org/en-US/docs/Web/API/WebRTC_API/Perfect_negotiation
	var offer = await pc.createOffer()
	await pc.setLocalDescription(offer)
	
	clientSay({type: MessageTypeOffer, description: pc.localDescription})
}

pc.onicecandidate = e => {
	if (e.candidate === null || e.candidate.candidate === "") {
		return
	}
	clientSay({type: MessageTypeICE, candidate: e.candidate})
}

pc.ontrack = e => {
	var el = document.createElement("video")
	el.srcObject = e.streams[0]
	el.autoplay = true
	el.controls = true
	document.getElementById("removeVideo").appendChild(el)
	e.streams[0].onremovetrack = e => {
		el.remove()
	}
}

pc.ondatachannel = e => {
	var el = document.createElement("div")
	el.innerText = e.channel.label
	document.getElementById("channels").appendChild(el)

	e.channel.onclose = e => {
		el.remove()
	}
}

document.getElementById("AddRemoteTrack").onclick = e => {
	clientSay({type: MessageTypeNewTrack})
}

document.getElementById("RemoveRemoteTrack").onclick = e => {
	clientSay({type: MessageTypeRemoveTrack})
}

document.getElementById("AddLocalTrack").onclick = e => {
	// Only video it is for logic
	navigator.mediaDevices.getUserMedia({video: true, audio: false})
	.then(stream  => {
		var el = document.createElement("video")
		el.srcObject = stream
		el.autoplay = true
		el.controls = true
		document.getElementById("localVideo").appendChild(el)

		stream.getTracks().forEach(track => {
			// Should be one track
			sender = pc.addTrack(track, stream)
			sender.el = el
		})
	})
	.catch(console.error)
}

document.getElementById("RemoveLocalTrack").onclick = e => {
	var senders = pc.getSenders()
	if (senders.length == 0) {
		return
	}
	var sender = senders[0]
	sender.el.remove()
	pc.removeTrack(sender)
}

document.getElementById("AddLocalChannel").onclick = e => {
	var channel = pc.createDataChannel("channel-local"+Math.random().toString(16))
	var el = document.createElement("div")
	el.innerText = channel.label
	document.getElementById("channels").appendChild(el)

	channel.onclose = e => {
		el.remove()
	}
	channels.push(channel)
}

document.getElementById("RemoveLocalChannel").onclick = e => {
	if (channels.length == 0) {
		return
	}
	channels.pop().close()
}

document.getElementById("AddRemoteChannel").onclick = e => {
	clientSay({type: MessageTypeRemoveChannel})
}

document.getElementById("RemoveRemoteChannel").onclick = e => {
	clientSay({type: MessageTypeAddChannel})
}
