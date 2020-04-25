
// Create peer conn
const pc = new RTCPeerConnection({
  iceServers: [
    {
      urls: 'stun:stun.l.google.com:19302'
    }
  ]
})

pc.oniceconnectionstatechange = e => {
  console.debug('connection state change', pc.iceConnectionState)
}
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

pc.onnegotiationneeded = e =>
  pc.createOffer().then(d => pc.setLocalDescription(d)).catch(console.error)

pc.ontrack = event => {
  console.log('Got track event', event)
  document.getElementById('serverVideo').srcObject = new MediaStream([event.track])
}

// Capture canvas streams and add to peer conn
const streams = [
  document.getElementById('canvasOne').captureStream(),
  document.getElementById('canvasTwo').captureStream(),
  document.getElementById('canvasThree').captureStream()
]
streams.forEach(stream => stream.getVideoTracks().forEach(track => pc.addTrack(track, stream)))

// Start circles
requestAnimationFrame(() => drawCircle(document.getElementById('canvasOne').getContext('2d'), '#006699', 0))
requestAnimationFrame(() => drawCircle(document.getElementById('canvasTwo').getContext('2d'), '#cf635f', 0))
requestAnimationFrame(() => drawCircle(document.getElementById('canvasThree').getContext('2d'), '#46c240', 0))

function drawCircle(ctx, color, angle) {
  // Background
  ctx.clearRect(0, 0, 200, 200)
  ctx.fillStyle = '#eeeeee'
  ctx.fillRect(0, 0, 200, 200)
  // Draw and fill in circle
  ctx.beginPath()
  const radius = 25 + 50 * Math.abs(Math.cos(angle))
  ctx.arc(100, 100, radius, 0, Math.PI * 2, false)
  ctx.closePath()
  ctx.fillStyle = color
  ctx.fill()
  // Call again
  requestAnimationFrame(() => drawCircle(ctx, color, angle + (Math.PI / 64)))
}

window.startSession = () => {
  const sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
  } catch (e) {
    alert(e)
  }
}