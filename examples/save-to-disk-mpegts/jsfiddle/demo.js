/* eslint-env browser */

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

const pc = new RTCPeerConnection({
  iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
})
const log = message => {
  document.getElementById('logs').innerHTML += message + '<br>'
}

navigator.mediaDevices.getUserMedia({ video: true, audio: false })
  .then(stream => {
    document.getElementById('preview').srcObject = stream
    stream.getVideoTracks().forEach(track => pc.addTrack(track, stream))
    return pc.createOffer()
  })
  .then(offer => pc.setLocalDescription(offer))
  .catch(log)

pc.oniceconnectionstatechange = () => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

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
