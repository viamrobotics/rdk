package stream

var viewHTML = `
<!DOCTYPE html>
<html>
<head>
  <title></title>
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
  let pc = new RTCPeerConnection({
  iceServers: %[2]s
})

pc.ontrack = function (event) {
  var el = document.createElement(event.track.kind)
  el.srcObject = event.streams[0]
  el.autoplay = true
  el.controls = false
  el.onclick = function(event) {
    // https://stackoverflow.com/a/288731/1497139
      bounds=this.getBoundingClientRect();
      console.log(bounds);
      var left=bounds.left;
      var top=bounds.top;
      var x = event.pageX - left;
      var y = event.pageY - top;
      var cw=this.clientWidth
      var ch=this.clientHeight
      var iw=this.videoWidth
      var ih=this.videoHeight
      var px=x/cw*iw
      var py=y/ch*ih
      console.log("click on "+this.tagName+" at pixel ("+px+","+py+") mouse pos ("+x+"," + y+ ") relative to boundingClientRect at ("+left+","+top+") client image size: "+cw+" x "+ch+" natural image size: "+iw+" x "+ih );
    actualDc.send(px+","+py);
}

  document.getElementById('remoteVideo_%[1]d').appendChild(el)
}

var dc = pc.createDataChannel("stuff", {id: 0});
var actualDc;
pc.ondatachannel = function(ev) {
  console.log('Data channel is created!');
  actualDc = ev.channel;
  ev.channel.onopen = function() {
    console.log('Data channel is open and ready to be used.');
  };
  ev.channel.onmessage = function (event) {
    console.log("received: " + event.data);
  };
};

pc.onsignalingstatechange = e => console.log(pc.signalingState)
pc.oniceconnectionstatechange = e => console.log(pc.iceConnectionState)
let sd;
pc.onicecandidate = event => {
  if (event.candidate === null) {
    fetch("/offer_%[1]d", {
      method: 'POST',
      mode: 'cors',
      body: btoa(JSON.stringify(pc.localDescription))
    }).then(response => response.text())
    .then(text => {
      sd = text;
      window.startSession();
    });
  }
}

// Offer to receive 1 audio, and 1 video track
pc.addTransceiver('video', {'direction': 'sendrecv'})
pc.addTransceiver('audio', {'direction': 'sendrecv'})

pc.createOffer().then(d => pc.setLocalDescription(d)).catch(console.log)

window.startSession = () => {
  if (sd === '') {
    return alert('Session Description must not be empty')
  }

  try {
    pc.setRemoteDescription(new RTCSessionDescription(JSON.parse(atob(sd))))
  } catch (e) {
    alert(e)
  }
}
}
`

var viewBody = `
Video<br />
<button onclick="start(); this.remove();">Start</button>
<div id="remoteVideo_%[1]d"></div> <br />
`
