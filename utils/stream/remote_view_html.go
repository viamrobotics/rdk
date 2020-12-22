package stream

var viewHTML = `
<!DOCTYPE html>
<html>
<head>
  <script type="text/javascript">
  ` + viewJS + `
  </script>
</head>
<body>
` + viewBody + `
</body>
</html>
`

// TODO(erd): refactor start session
var viewJS = `
const start = function() {
  let peerConnection = new RTCPeerConnection({
    iceServers: %[2]s
  });

  const calculateClick = (el, event) => {
    // https://stackoverflow.com/a/288731/1497139
    bounds = el.getBoundingClientRect();
    let left =bounds.left;
    let top =bounds.top;
    let x = event.pageX - left;
    let y = event.pageY - top;
    let cw = el.clientWidth
    let ch = el.clientHeight
    let iw = el.videoWidth
    let ih = el.videoHeight
    let px = x/cw*iw
    let py = y/ch*ih
    return {x: px, y: py};
  }

  let clickChannel;

  peerConnection.ontrack = event => {
    var videoElement = document.createElement(event.track.kind);
    videoElement.srcObject = event.streams[0];
    videoElement.autoplay = true;
    videoElement.controls = false;
    videoElement.playsInline = true;
    videoElement.onclick = events => {
      coords = calculateClick(videoElement, event);
      clickChannel.send(coords.x + "," + coords.y);
    }
    document.getElementById('remoteVideo_%[1]d').appendChild(videoElement)
  }

  peerConnection.ondatachannel = event => {
    clickChannel = event.channel;
  };

  peerConnection.onicecandidate = event => {
    if (event.candidate !== null) {
      return;
    }
    fetch("/offer_%[1]d", {
      method: 'POST',
      mode: 'cors',
      body: btoa(JSON.stringify(peerConnection.localDescription))
    })
    .then(response => response.text())
    .then(text => {
      try {
        peerConnection.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(text))));
      } catch(e) {
        console.log(e);
      }
    });
  }
  peerConnection.onsignalingstatechange = () => console.log(peerConnection.signalingState);
  peerConnection.oniceconnectionstatechange = () => console.log(peerConnection.iceConnectionState);

  // set up offer
  peerConnection.createDataChannel("click", {id: 0});
  peerConnection.addTransceiver('video', {'direction': 'sendrecv'});
  peerConnection.createOffer()
    .then(desc => peerConnection.setLocalDescription(desc))
    .catch(console.log);
}
`

var viewBody = `
Video<br />
<button onclick="start(); this.remove();">Start</button>
<div id="remoteVideo_%[1]d"></div> <br />
`
