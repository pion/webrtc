/* eslint-env browser */

// SPDX-FileCopyrightText: 2026 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

const pc = new RTCPeerConnection({ iceServers: [{ urls: 'stun:stun.l.google.com:19302' }] })

function log (value) {
  console.log(value)
  const line = document.createElement('pre')
  line.innerText = value.toString()
  document.getElementById('logs').appendChild(line)
}

// Reorders available codecs so that the selected codec is first on the list
function codecs () {
  const option = document.getElementById('codec').value.toLowerCase()
  const caps = RTCRtpSender.getCapabilities('video')?.codecs ?? []

  const primary = caps.filter((c) => c.mimeType.toLowerCase() === `video/${option}`)
  if (primary.length === 0) {
    alert('Unsupported codec selected')
    throw new DOMException('Unsupported codec')
  }
  const primaryPayloads = new Set(primary.map((c) => c.preferredPayloadType))
  const pairedRtx = caps.filter(
    (c) => c.mimeType.toLowerCase() === 'video/rtx' && c.sdpFmtpLine?.includes('apt=') && primaryPayloads.has(Number(c.sdpFmtpLine.split('apt=')[1]))
  )

  const rest = caps.filter((c) => !primary.includes(c) && !pairedRtx.includes(c))
  const ordered = [...primary, ...pairedRtx, ...rest]
  console.log('Codec preferences', ordered)
  return ordered
}

async function start () {
  log('Starting...')
  const screenSrc = await navigator.mediaDevices.getDisplayMedia({ video: { frameRate: 30, height: 360 } })
  const video = document.getElementById('screen')
  video.hidden = false
  video.srcObject = screenSrc

  const trans = pc.addTransceiver(screenSrc.getVideoTracks()[0], { direction: 'sendrecv' })
  trans.setCodecPreferences(codecs())

  const offer = await pc.createOffer()
  await pc.setLocalDescription(offer)

  log('Gathering ICE candidates')

  await new Promise((resolve) => {
    pc.onicecandidate = (ev) => {
      if (ev.candidate == null) {
        resolve()
      }
    }
    if (pc.iceGatheringState === 'complete') {
      resolve()
    }
  })

  log('Done gathering ICE candidates')

  log('Sending SDP')
  const resp = await fetch('/sdp', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json'
    },
    body: JSON.stringify(pc.localDescription)
  })
  const sdp = await resp.json()

  pc.setRemoteDescription(sdp)
}

pc.addEventListener('iceconnectionstatechange', () => {
  console.log('ICE connection state', pc.iceConnectionState)
})

pc.ontrack = (t) => {
  log('New track received')
  const received = document.getElementById('received')
  received.hidden = false
  received.srcObject = t.streams[0]
  pc.getStats().then((stats) => {
    const codec = Array.from(stats.values()).find((e) => e.type === 'codec')
    log(`New track codec: ${codec.mimeType}`)
  })
}

document.getElementById('start').onclick = start
