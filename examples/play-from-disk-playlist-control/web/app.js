/* eslint-env browser */

// SPDX-FileCopyrightText: 2024 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

let pc = null
let playlistChannel = null
let started = false

const logs = document.getElementById('logs')
const nowPlayingEl = document.getElementById('nowPlaying')
const playlistEl = document.getElementById('playlist')
const startButton = document.getElementById('startButton')
const audio = document.getElementById('remoteAudio')

const log = msg => {
  logs.innerHTML += `${msg}<br>`
  logs.scrollTop = logs.scrollHeight
}

async function startSession () {
  if (started) {
    return
  }
  started = true
  startButton.disabled = true
  log('Creating PeerConnection...')

  pc = new RTCPeerConnection({
    iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
  })

  pc.createDataChannel('sctp-bootstrap')
  pc.oniceconnectionstatechange = () => log(`ICE state: ${pc.iceConnectionState}`)
  pc.onconnectionstatechange = () => log(`Peer state: ${pc.connectionState}`)
  pc.ontrack = event => {
    audio.srcObject = event.streams[0]
    audio.play().catch(() => {})
  }
  pc.ondatachannel = event => {
    if (event.channel.label !== 'playlist') {
      return
    }
    playlistChannel = event.channel
    playlistChannel.onopen = () => log('playlist DataChannel open')
    playlistChannel.onclose = () => log('playlist DataChannel closed')
    playlistChannel.onmessage = e => handleMessage(e.data)
  }

  pc.addTransceiver('audio', { direction: 'recvonly' })

  try {
    const offer = await pc.createOffer()
    await pc.setLocalDescription(offer)
    log(`Sending offer (${pc.localDescription.sdp.length} bytes)`)

    const res = await fetch('/whep', {
      method: 'POST',
      headers: { 'Content-Type': 'application/sdp' },
      body: pc.localDescription.sdp
    })
    if (!res.ok) {
      const body = await res.text()
      throw new Error(`whep failed: ${res.status} ${body}`)
    }
    const answerSDP = await res.text()
    if (!answerSDP) {
      throw new Error('no SDP answer from server')
    }
    await pc.setRemoteDescription({ type: 'answer', sdp: answerSDP })
    log('Answer applied. Waiting for media and playlist...')
  } catch (err) {
    log(`Error during negotiation: ${err}`)
  }
}

function sendPrev () {
  sendRawCommand('prev')
}

function sendNext () {
  sendRawCommand('next')
}

function sendList () {
  sendRawCommand('list')
}

function sendCommand () {
  const value = document.getElementById('commandInput').value
  if (value.trim() === '') {
    return
  }
  sendRawCommand(value)
}

function sendRawCommand (text) {
  if (!playlistChannel || playlistChannel.readyState !== 'open') {
    log('playlist channel not open yet')
    return
  }

  playlistChannel.send(text)
}

function handleMessage (data) {
  const lines = data.trim().split('\n')
  const playlist = []
  let current = null
  let now = null

  lines.forEach(line => {
    const parts = line.split('|')
    if (parts.length === 0) {
      return
    }
    switch (parts[0]) {
      case 'playlist':
        current = Number(parts[1] || 0)
        break
      case 'track':
        playlist.push({
          index: Number(parts[1] || 0),
          serial: Number(parts[2] || 0),
          duration_ms: Number(parts[3] || 0),
          title: parts[4] || '',
          artist: parts[5] || ''
        })
        break
      case 'now':
        now = {
          index: Number(parts[1] || 0),
          serial: Number(parts[2] || 0),
          channels: Number(parts[3] || 0),
          sample_rate: Number(parts[4] || 0),
          duration_ms: Number(parts[5] || 0),
          title: parts[6] || '',
          artist: parts[7] || '',
          vendor: parts[8] || '',
          comments: (parts[9] || '').split(',').filter(Boolean).map(s => {
            const [k, v] = s.split('=')
            return { key: k, value: v }
          })
        }
        break
      default:
        log(`Message: ${line}`)
    }
  })

  if (playlist.length > 0) {
    renderPlaylist({ tracks: playlist, current })
  }
  if (now) {
    renderNowPlaying(now)
  }
}

function renderPlaylist (message) {
  playlistEl.innerHTML = ''
  message.tracks.forEach(track => {
    const li = document.createElement('li')
    li.innerText = `${track.index + 1}. ${track.title || '(untitled)'} — ${track.artist || 'unknown artist'} (${prettyDuration(track.duration_ms)})`
    if (track.index === message.current) {
      li.classList.add('current')
    }
    playlistEl.appendChild(li)
  })

  if (message.hint) {
    log(message.hint)
  }
}

function renderNowPlaying (track) {
  const title = track.title || '(untitled)'
  const artist = track.artist || 'unknown artist'
  const vendor = track.vendor ? `<div class="meta">Vendor: ${track.vendor}</div>` : ''
  const channels = track.channels || '?'
  const sampleRate = track.sample_rate || '?'
  const comments = (track.comments || []).map(c => `<div class="meta">${c.key}: ${c.value}</div>`).join('')

  nowPlayingEl.innerHTML = `
    <div class="label">Now playing</div>
    <div class="track">${title}</div>
    <div class="artist">${artist}</div>
    <div class="meta">Serial: ${track.serial} | Channels: ${channels} | Sample rate: ${sampleRate}</div>
    <div class="meta">Duration: ${prettyDuration(track.duration_ms)}</div>
    ${vendor}
    ${comments}
  `
}

function prettyDuration (ms) {
  if (!ms || ms < 0) {
    return 'unknown'
  }
  const totalSeconds = Math.round(ms / 1000)
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${seconds.toString().padStart(2, '0')}`
}

window.startSession = startSession
window.sendPrev = sendPrev
window.sendNext = sendNext
window.sendList = sendList
window.sendCommand = sendCommand
