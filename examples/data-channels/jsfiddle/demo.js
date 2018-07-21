/* eslint-env browser */

let pc = new RTCPeerConnection()
let log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

let sendChannel = pc.createDataChannel('foo')
sendChannel.onclose = () => console.log('sendChannel has closed')
sendChannel.onopen = () => console.log('sendChannel has opened')
sendChannel.onmessage = e => log(`sendChannel got '${e.data}'`)

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)

pc.onnegotiationneeded = e =>
  pc.createOffer({ }).then(d => {
    document.getElementById('localSessionDescription').value = btoa(d.sdp)
    return pc.setLocalDescription(d)
  }).catch(log)

window.sendMessage = () => {
  let message = document.getElementById('message').value
  if (message === '') {
    return alert('Message must not be empty')
  }

  sendChannel.send(message)
}

window.startSession = () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: atob(sd)}))
  } catch (e) {
    alert(e)
  }
}
