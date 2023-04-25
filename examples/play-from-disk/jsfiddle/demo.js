/* eslint-env browser */

// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

const pc = new RTCPeerConnection({
  iceServers: [{
    urls: 'stun:stun.l.google.com:19302'
  }]
})
const log = msg => {
  document.getElementById('div').innerHTML += msg + '<br>'
}

pc.ontrack = function (event) {
  const el = document.createElement(event.track.kind)
  el.srcObject = event.streams[0]
  el.autoplay = true
  el.controls = true

  document.getElementById('remoteVideos').appendChild(el)
}

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

// Offer to receive 1 audio, and 1 video track
pc.addTransceiver('video', {
  direction: 'sendrecv'
})
pc.addTransceiver('audio', {
  direction: 'sendrecv'
})

pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)

window.startSession = () => {
  const sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    pc.setRemoteDescription(JSON.parse(atob(sd)))
  } catch (e) {
    alert(e)
  }
}

window.copySessionDescription = () => {
  const browserSessionDescription = document.getElementById('localSessionDescription')

  browserSessionDescription.focus()
  browserSessionDescription.select()

  try {
    const successful = document.execCommand('copy')
    const msg = successful ? 'successful' : 'unsuccessful'
    log('Copying SessionDescription was ' + msg)
  } catch (err) {
    log('Oops, unable to copy SessionDescription ' + err)
  }
}
