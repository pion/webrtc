/* eslint-env browser */

let pc = new RTCPeerConnection()
let log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

// let sendChannel = pc.createDataChannel('foo')
// sendChannel.onclose = () => console.log('sendChannel has closed')
// sendChannel.onopen = () => console.log('sendChannel has opened')
// sendChannel.onmessage = e => log(`sendChannel got '${e.data}'`)

pc.onsignalingstatechange = e => log(pc.signalingState)
pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(pc.localDescription.sdp)
  }
}

pc.ondatachannel = e => {
  log('got dc')
  dc = e.channel
  dc.onclose = () => console.log('dc has closed')
  dc.onopen = () => console.log('dc has opened')
  dc.onmessage = e => log(`dc got '${e.data}'`)
  window.sendMessage = () => {
    let message = document.getElementById('message').value
    if (message === '') {
      return alert('Message must not be empty')
    }

    dc.send(message)
  }
}

// pc.onnegotiationneeded = e =>
//   pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)



window.startSession = () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  log(atob(sd))
  pc.setRemoteDescription(new RTCSessionDescription({type: 'offer', sdp: atob(sd)})).catch(log)

  log("ss", pc.signalingState)
  pc.createAnswer().then(d => pc.setLocalDescription(d)).catch(log)

}
