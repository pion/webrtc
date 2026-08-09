/* eslint-env browser */

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

const pc = new RTCPeerConnection({
  iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
})
const log = message => {
  document.getElementById('logs').innerHTML += message + '<br>'
}

pc.ontrack = event => {
  const video = document.createElement('video')
  video.srcObject = event.streams[0]
  video.autoplay = true
  video.controls = true
  document.getElementById('remoteVideo').appendChild(video)
}
pc.oniceconnectionstatechange = () => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

pc.addTransceiver('video', { direction: 'recvonly' })
pc.createOffer()
  .then(offer => pc.setLocalDescription(offer))
  .catch(log)

window.startSession = () => {
  const description = document.getElementById('remoteSessionDescription').value
  if (description === '') {
    return alert('Session Description must not be empty')
  }
  pc.setRemoteDescription(JSON.parse(atob(description))).catch(log)
}

window.copySessionDescription = () => {
  navigator.clipboard.writeText(document.getElementById('localSessionDescription').value)
    .then(() => log('Browser Session Description copied'))
    .catch(log)
}
