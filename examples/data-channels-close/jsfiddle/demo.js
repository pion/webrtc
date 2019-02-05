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

window.createDataChannel = name => {
  let dc = pc.createDataChannel(name)
  let fullName = `Data channel '${dc.label}' (${dc.id})`
  dc.onopen = () => {
    log(`${fullName}: has opened`)
    dc.onmessage = e => log(`${fullName}: '${e.data}'`)

    let ul = document.getElementById('ul-open')
    let li = document.createElement('li')
    li.appendChild(document.createTextNode(`${fullName}: `))

    let btnSend = document.createElement('BUTTON')
    btnSend.appendChild(document.createTextNode('Send message'))
    btnSend.onclick = () => {
      let message = document.getElementById('message').value
      if (message === '') {
        return alert('Message must not be empty')
      }

      dc.send(message)
    }
    li.appendChild(btnSend)

    let btnClose = document.createElement('BUTTON')
    btnClose.appendChild(document.createTextNode('Close'))
    btnClose.onclick = () => {
      dc.close()
      ul.removeChild(li)
    }
    li.appendChild(btnClose)

    dc.onclose = () => {
      log(`${fullName}: closed.`)
      ul.removeChild(li)
    }

    ul.appendChild(li)
  }
}

pc.oniceconnectionstatechange = e => log(`ICE state: ${pc.iceConnectionState}`)
pc.onicecandidate = event => {
  if (event.candidate === null) {
    document.getElementById('localSessionDescription').value = btoa(JSON.stringify(pc.localDescription))
  }
}

pc.onnegotiationneeded = e =>
  pc.createOffer().then(d => pc.setLocalDescription(d)).catch(log)

window.startSession = () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
  } catch (e) {
    alert(e)
  }
}
