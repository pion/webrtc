// This file adds RTCPeerConnection to the global context, making Node.js more
// closely match the browser API for WebRTC.

const wrtc = require('wrtc')

global.window = {
  RTCPeerConnection: wrtc.RTCPeerConnection
}

global.RTCPeerConnection = wrtc.RTCPeerConnection
