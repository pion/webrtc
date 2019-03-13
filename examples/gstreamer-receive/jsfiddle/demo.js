/* eslint-env browser */

let pc = new RTCPeerConnection({
  iceServers: [
    {
      urls: 'stun:stun.l.google.com:19302'
    }
  ]
})
let log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}
let displayVideo = video => {
  var el = document.createElement('video')
  el.srcObject = video
  el.autoplay = true
  el.muted = true
  el.width = 160
  el.height = 120

  document.getElementById('localVideos').appendChild(el)
  return video
}

navigator.mediaDevices.getUserMedia({ video: true, audio: true })
  .then(stream => {
    pc.addStream(displayVideo(stream))
    pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)
  }).catch(log)

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

window.startSession = () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
  } catch (e) {
    alert(e)
  }
}

window.addDisplayCapture = () => {
  navigator.mediaDevices.getDisplayMedia().then(stream => {
    document.getElementById('displayCapture').disabled = true
    pc.addStream(displayVideo(stream))
    pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)
  })
}
