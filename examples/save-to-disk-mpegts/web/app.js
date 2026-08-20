/* eslint-env browser */

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

let pc = null
let stream = null
let sessionURL = null
let stopping = false

const logs = document.getElementById('logs')
const preview = document.getElementById('preview')
const startButton = document.getElementById('startButton')
const stopButton = document.getElementById('stopButton')

const log = message => {
  const entry = document.createElement('div')
  entry.textContent = message
  logs.appendChild(entry)
}

const waitForICEGathering = connection => {
  if (connection.iceGatheringState === 'complete') {
    return Promise.resolve()
  }

  return new Promise(resolve => {
    const checkState = () => {
      if (connection.iceGatheringState === 'complete') {
        connection.removeEventListener('icegatheringstatechange', checkState)
        resolve()
      }
    }
    connection.addEventListener('icegatheringstatechange', checkState)
  })
}

const preferH264 = transceiver => {
  const codecs = RTCRtpSender.getCapabilities('video')?.codecs ?? []
  const h264 = codecs.filter(codec => codec.mimeType.toLowerCase() === 'video/h264')
  if (h264.length === 0) {
    throw new Error('this browser does not support H.264 WebRTC')
  }
  transceiver.setCodecPreferences(h264)
}

const closeLocalSession = () => {
  stream?.getTracks().forEach(track => track.stop())
  pc?.close()
  preview.srcObject = null
  stream = null
  pc = null
}

const deleteRemoteSession = async () => {
  if (sessionURL === null) {
    return
  }

  const response = await fetch(sessionURL, { method: 'DELETE' })
  if (!response.ok) {
    throw new Error(`stop failed: ${response.status} ${await response.text()}`)
  }
  sessionURL = null
}

async function startSession () {
  if (pc !== null || stopping) {
    return
  }
  startButton.disabled = true
  try {
    stream = await navigator.mediaDevices.getUserMedia({ video: true, audio: false })
    preview.srcObject = stream

    pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
    })
    pc.oniceconnectionstatechange = () => log(`ICE state: ${pc.iceConnectionState}`)
    pc.onconnectionstatechange = () => log(`Peer state: ${pc.connectionState}`)

    const transceiver = pc.addTransceiver(stream.getVideoTracks()[0], {
      direction: 'sendonly',
      streams: [stream]
    })
    preferH264(transceiver)

    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    log('Gathering ICE candidates...')
    await waitForICEGathering(pc)

    const response = await fetch('/whip', {
      method: 'POST',
      headers: { 'Content-Type': 'application/sdp' },
      body: pc.localDescription.sdp
    })
    if (!response.ok) {
      throw new Error(`WHIP failed: ${response.status} ${await response.text()}`)
    }
    sessionURL = response.headers.get('Location') || '/whip'

    const answerSDP = await response.text()
    if (answerSDP === '') {
      throw new Error('empty SDP answer')
    }
    await pc.setRemoteDescription({ type: 'answer', sdp: answerSDP })
    stopButton.disabled = false
    log('Recording started')
  } catch (error) {
    log(`Recording failed: ${error}`)
    try {
      await deleteRemoteSession()
    } catch (stopError) {
      log(`Cleanup failed: ${stopError}`)
    }
    closeLocalSession()
    startButton.disabled = false
  }
}

async function stopSession () {
  if (stopping || pc === null) {
    return
  }
  stopping = true
  stopButton.disabled = true
  stream?.getTracks().forEach(track => track.stop())

  try {
    await deleteRemoteSession()
    log('Recording saved')
  } catch (error) {
    log(`Failed to stop recording: ${error}`)
  } finally {
    closeLocalSession()
    stopping = false
    startButton.disabled = false
  }
}

window.addEventListener('pagehide', () => {
  if (sessionURL !== null) {
    fetch(sessionURL, { method: 'DELETE', keepalive: true }).catch(() => {})
  }
  closeLocalSession()
})

window.startSession = startSession
window.stopSession = stopSession
