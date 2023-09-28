/* eslint-env browser */

// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// cipherKey that video is encrypted with
const cipherKey = 0xAA

const pc = new RTCPeerConnection({ encodedInsertableStreams: true, forceEncodedVideoInsertableStreams: true })
const log = msg => {
  document.getElementById('div').innerHTML += msg + '<br>'
}

// Offer to receive 1 video
const transceiver = pc.addTransceiver('video')

// The API has seen two iterations, support both
// In the future this will just be `createEncodedStreams`
const receiverStreams = getInsertableStream(transceiver)

// boolean controlled by checkbox to enable/disable encryption
let applyDecryption = true
window.toggleDecryption = () => {
  applyDecryption = !applyDecryption
}

// Loop that is called for each video frame
const reader = receiverStreams.readable.getReader()
const writer = receiverStreams.writable.getWriter()
reader.read().then(function processVideo ({ done, value }) {
  const decrypted = new DataView(value.data)

  if (applyDecryption) {
    for (let i = 0; i < decrypted.buffer.byteLength; i++) {
      decrypted.setInt8(i, decrypted.getInt8(i) ^ cipherKey)
    }
  }

  value.data = decrypted.buffer
  writer.write(value)
  return reader.read().then(processVideo)
})

// Fire when remote video arrives
pc.ontrack = function (event) {
  document.getElementById('remote-video').srcObject = event.streams[0]
  document.getElementById('remote-video').style = ''
}

// Populate SDP field when finished gathering
pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}
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

// DOM code to show banner if insertable streams not supported
let insertableStreamsSupported = true
const updateSupportBanner = () => {
  const el = document.getElementById('no-support-banner')
  if (insertableStreamsSupported && el) {
    el.style = 'display: none'
  }
}
document.addEventListener('DOMContentLoaded', updateSupportBanner)

// Shim to support both versions of API
function getInsertableStream (transceiver) {
  let insertableStreams = null
  if (transceiver.receiver.createEncodedVideoStreams) {
    insertableStreams = transceiver.receiver.createEncodedVideoStreams()
  } else if (transceiver.receiver.createEncodedStreams) {
    insertableStreams = transceiver.receiver.createEncodedStreams()
  }

  if (!insertableStreams) {
    insertableStreamsSupported = false
    updateSupportBanner()
    throw new Error('Insertable Streams are not supported')
  }

  return insertableStreams
}
