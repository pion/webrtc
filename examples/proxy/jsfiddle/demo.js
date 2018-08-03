/* eslint-env browser */

let pc = new RTCPeerConnection()
var log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

navigator.mediaDevices.getUserMedia({video: true, audio: true})
  .then(stream => pc.addStream(document.getElementById('video1').srcObject = stream))
  .catch(log)

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)

pc.onnegotiationneeded = e =>
  pc.createOffer().then(d => {
    document.getElementById('localSessionDescription').value = btoa(d.sdp)
    return pc.setLocalDescription(d)
  }).catch(log)

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
