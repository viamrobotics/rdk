interface ReadyPeer {
	pc: RTCPeerConnection;
	dc: RTCDataChannel; 
}

export async function newPeerConnectionForClient(): Promise<ReadyPeer> {
	const peerConnection = new RTCPeerConnection({
		// TODO(https://github.com/viamrobotics/core/issues/81): Use Viam STUN/TURN.
		iceServers: [
			{
				urls: 'stun:stun.erdaniels.com'
			},
			{
				// TODO(https://github.com/viamrobotics/core/issues/81): Use Viam STUN/TURN.
				urls: 'turn:stun.erdaniels.com',
				username: "username",
				credentialType: "password",
				credential: "password"
			}
		]
	});

	let pResolve: (value: ReadyPeer) => void;
	const result = new Promise<ReadyPeer>(resolve => {
		pResolve = resolve;
	})
	const dataChannel = peerConnection.createDataChannel("data", {
		id: 0,
		negotiated: true,
		ordered: true
	});
	dataChannel.binaryType = "arraybuffer";

	peerConnection.onicecandidate = async event => {
		if (event.candidate !== null) {
			return;
		}
		pResolve({ pc: peerConnection, dc: dataChannel });
	}

	// set up offer
	const offerDesc = await peerConnection.createOffer();
	try {
		peerConnection.setLocalDescription(offerDesc)
	} catch (e) {
		console.error(e);
	}
	return result;
}
