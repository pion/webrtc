/* eslint-env browser */

let pc = new RTCPeerConnection({
  iceServers: [
    {
      urls: 'stun:stun.l.google.com:19302'
    }
  ]
})
let log = msg => {
  document.getElementById('logs').innerHTML += msg + '<br>'
}

pc.onsignalingstatechange = e => log(pc.signalingState)
pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

pc.ondatachannel = e => {
  let dc = e.channel
  log('New DataChannel ' + dc.label)
  dc.onclose = () => console.log('dc has closed')
  dc.onopen = () => console.log('dc has opened')
  dc.onmessage = e => log(`Message from DataChannel '${dc.label}' payload '${e.data}'`)
  window.sendMessage = () => {
    let message = document.getElementById('message').value
    if (message === '') {
      return alert('Message must not be empty')
    }

    dc.send(message)
  }
}

window.startSession = () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd)))).catch(log)
  pc.createAnswer().then(d => pc.setLocalDescription(d)).catch(log)
}
