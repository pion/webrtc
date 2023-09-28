/* eslint-env browser */

// SPDX-FileCopyrightText: 2023 The Pion community <https://pion.ly>
// SPDX-License-Identifier: MIT

// Create peer conn
const pc = new RTCPeerConnection({
  iceServers: [{
    urls: 'stun:stun.l.google.com:19302'
  }]
})

pc.oniceconnectionstatechange = (e) => {
  console.log('connection state change', pc.iceConnectionState)
}
pc.onicecandidate = (event) => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(
      JSON.stringify(pc.localDescription)
    )
  }
}

pc.onnegotiationneeded = (e) =>
  pc
    .createOffer()
    .then((d) => pc.setLocalDescription(d))
    .catch(console.error)

pc.ontrack = (event) => {
  console.log('Got track event', event)
  const video = document.createElement('video')
  video.srcObject = event.streams[0]
  video.autoplay = true
  video.width = '500'
  const label = document.createElement('div')
  label.textContent = event.streams[0].id
  document.getElementById('serverVideos').appendChild(label)
  document.getElementById('serverVideos').appendChild(video)
}

navigator.mediaDevices
  .getUserMedia({
    video: {
      width: {
        ideal: 4096
      },
      height: {
        ideal: 2160
      },
      frameRate: {
        ideal: 60,
        min: 10
      }
    },
    audio: false
  })
  .then((stream) => {
    document.getElementById('browserVideo').srcObject = stream
    pc.addTransceiver(stream.getVideoTracks()[0], {
      direction: 'sendonly',
      streams: [stream],
      sendEncodings: [
        // for firefox order matters... first high resolution, then scaled resolutions...
        {
          rid: 'f'
        },
        {
          rid: 'h',
          scaleResolutionDownBy: 2.0
        },
        {
          rid: 'q',
          scaleResolutionDownBy: 4.0
        }
      ]
    })
    pc.addTransceiver('video')
    pc.addTransceiver('video')
    pc.addTransceiver('video')
  })

window.startSession = () => {
  const sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    console.log('answer', JSON.parse(atob(sd)))
    pc.setRemoteDescription(JSON.parse(atob(sd)))
  } catch (e) {
    alert(e)
  }
}

window.copySDP = () => {
  const browserSDP = document.getElementById('localSessionDescription')

  browserSDP.focus()
  browserSDP.select()

  try {
    const successful = document.execCommand('copy')
    const msg = successful ? 'successful' : 'unsuccessful'
    console.log('Copying SDP was ' + msg)
  } catch (err) {
    console.log('Unable to copy SDP ' + err)
  }
}
