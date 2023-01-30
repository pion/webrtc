/* eslint-env browser */

const pc = new RTCPeerConnection({
  iceServers: [
    {
      urls: 'stun:stun.l.google.com:19302'
    }
  ]
})
const log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

const sendChannel = pc.createDataChannel('foo')
sendChannel.onclose = () => console.log('sendChannel has closed')
sendChannel.onopen = () => console.log('sendChannel has opened')
sendChannel.onmessage = e => log(`Message from DataChannel '${sendChannel.label}' payload '${e.data}'`)

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
// after dom is loaded get the candidate
pc.onicecandidate = event => {
  if (event.candidate === null) {
    const offer = btoa(JSON.stringify(pc.localDescription))
    // make sure the DOM is ready
    if (document.readyState === 'complete') {
      const browserSDP = document.getElementById('localSessionDescription')
      browserSDP.value = offer
    } else
      document.addEventListener('DOMContentLoaded', () => {
        const browserSDP = document.getElementById('localSessionDescription')
        browserSDP.value = offer
      })
  }
}

pc.onnegotiationneeded = e =>
  pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)

window.sendMessage = () => {
  const message = document.getElementById('message').value
  if (message === '') {
    return alert('Message must not be empty')
  }

  sendChannel.send(message)
}

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

window.copySDP = () => {
  const browserSDP = document.getElementById('localSessionDescription')

  browserSDP.focus()
  browserSDP.select()

  try {
    const successful = document.execCommand('copy')
    const msg = successful ? 'successful' : 'unsuccessful'
    log('Copying SDP was ' + msg)
  } catch (err) {
    log('Unable to copy SDP ' + err)
  }
}
