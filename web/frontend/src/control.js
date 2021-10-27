const { grpc } = require("@improbable-eng/grpc-web");
window.robotApi = require('proto/api/v1/robot_pb.js');
const { RobotServiceClient } = require('proto/api/v1/robot_pb_service.js');
window.metadataApi = require('proto/api/service/v1/metadata_pb.js');
const { MetadataServiceClient } = require('proto/api/service/v1/metadata_pb_service.js');
const { dial } = require("@viamrobotics/rpc");
window.THREE = require("three/build/three.module.js")
window.pcdLib = require("three/examples/jsm/loaders/PCDLoader.js")
window.orbitLib = require("three/examples/jsm/controls/OrbitControls.js")
const rtcConfig = {
	iceServers: [
		{
			urls: 'stun:global.stun.twilio.com:3478?transport=udp'
		}
	]
}

if (window.webrtcAdditionalICEServers) {
	rtcConfig.iceServers = rtcConfig.iceServers.concat(window.webrtcAdditionalICEServers);
}

let pResolve, pReject;
window.robotServiceReady = new Promise((resolve, reject) => {
	pResolve = resolve;
	pReject = reject;
})
window.reconnect = async () => undefined;
if (window.webrtcEnabled) {
	let connect = async () => {
		try {
			let cc = await dial(window.webrtcSignalingAddress, window.webrtcHost, rtcConfig);
			window.robotService = new RobotServiceClient(window.webrtcHost, { transport: cc.transportFactory() });
			window.metadataService = new MetadataServiceClient(window.webrtcHost, { transport: cc.transportFactory() });
		} catch (e) {
			console.error("error dialing:", e);
			throw e;
		}
	}
	connect().then(pResolve).catch(pReject);
	window.reconnect = connect;
} else {
	const url = `${location.protocol}//${location.hostname}${location.port ? ':' + location.port : ''}`;
	window.robotService = new RobotServiceClient(url);
	window.metadataService = new MetadataServiceClient(url);
	pResolve(undefined);
}

