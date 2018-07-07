/* eslint-env browser */

let pc = new RTCPeerConnection()
let log = msg => {
  document.getElementById('div').innerHTML += msg + '<br>'
}

pc.ontrack = function (event) {
  var el = document.createElement(event.track.kind)
  el.srcObject = event.streams[0]
  el.autoplay = true
  el.controls = true

  document.getElementById('remoteVideos').appendChild(el)
}

pc.oniceconnectionstatechange = e => log(pc.iceConnectionState)
pc.createOffer({offerToReceiveVideo: true, offerToReceiveAudio: true}).then(d => {
  document.getElementById('localSessionDescription').value = btoa(d.sdp)
  return pc.setLocalDescription(d)
}).catch(log)

window.startSession = () => {
  let sd = document.getElementById('remoteSessionDescription').value
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    pc.setRemoteDescription(new RTCSessionDescription({type: 'answer', sdp: atob(sd)}))
  } catch (e) {
    alert(e)
  }
}
