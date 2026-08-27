/* eslint-env browser */

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

let pc = null
let started = false

const logs = document.getElementById('logs')
const video = document.getElementById('remoteVideo')
const startButton = document.getElementById('startButton')

const log = message => {
  logs.innerHTML += `${message}<br>`
}

const waitForICEGathering = () => {
  if (pc.iceGatheringState === 'complete') {
    return Promise.resolve()
  }

  return new Promise(resolve => {
    const checkState = () => {
      if (pc.iceGatheringState === 'complete') {
        pc.removeEventListener('icegatheringstatechange', checkState)
        resolve()
      }
    }
    pc.addEventListener('icegatheringstatechange', checkState)
  })
}

async function startSession () {
  if (started) {
    return
  }
  started = true
  startButton.disabled = true

  pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
  })
  pc.oniceconnectionstatechange = () => log(`ICE state: ${pc.iceConnectionState}`)
  pc.onconnectionstatechange = () => log(`Peer state: ${pc.connectionState}`)
  pc.ontrack = event => {
    video.srcObject = event.streams[0]
    video.play().catch(() => {})
  }
  pc.addTransceiver('video', { direction: 'recvonly' })

  try {
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    log('Gathering ICE candidates...')
    await waitForICEGathering()

    const response = await fetch('/whep', {
      method: 'POST',
      headers: { 'Content-Type': 'application/sdp' },
      body: pc.localDescription.sdp
    })
    if (!response.ok) {
      throw new Error(`WHEP failed: ${response.status} ${await response.text()}`)
    }

    const answerSDP = await response.text()
    if (answerSDP === '') {
      throw new Error('empty SDP answer')
    }
    await pc.setRemoteDescription({ type: 'answer', sdp: answerSDP })
    log('Answer applied; waiting for video...')
  } catch (error) {
    log(`Negotiation failed: ${error}`)
    started = false
    startButton.disabled = false
  }
}

window.startSession = startSession
