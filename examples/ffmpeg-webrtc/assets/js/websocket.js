var ws_socket = null
var session_id = undefined

function ws_connect(host) {
  return new Promise(function(resolve, reject) {
    if(!window["WebSocket"]){
      reject("error:browser does not support web sockets.")
    }

    ws_socket = new WebSocket('ws://'+host+'/ws')

    ws_socket.onopen = () => {
      resolve(ws_socket)
      console.log("ws successfully opened")
    }

    ws_socket.onerror = (err) => {
      console.log("error in ws communication")
      reject(err)
    }

    ws_socket.onclose = () => {
      console.log("ws closed")
    }
  })
}
