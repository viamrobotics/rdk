const { grpc } = require("@improbable-eng/grpc-web");
window.robotApi = require('proto/api/v1/robot_pb.js');
const { RobotServiceClient } = require('proto/api/v1/robot_pb_service.js');
const { dial } = require("rpc");
window.THREE = require("three/build/three.module.js")
window.pcdLib = require("three/examples/jsm/loaders/PCDLoader.js")
window.orbitLib = require("three/examples/jsm/controls/OrbitControls.js")

let pResolve, pReject;
window.robotServiceReady = new Promise((resolve, reject) => {
	pResolve = resolve;
	pReject = reject;
})
if (window.webrtcEnabled) {
	dial(window.webrtcSignalingAddress, window.webrtcHost).then(cc => {
		window.robotService = new RobotServiceClient(window.webrtcHost, { transport: cc.transportFactory() });
		pResolve(undefined);
	}).catch(e => {
		console.error("error dialing:", e);
		pReject(e);
	})
} else {
	const url = `${location.protocol}//${location.hostname}${location.port ? ':' + location.port : ''}`;
	window.robotService = new RobotServiceClient(url);
	pResolve(undefined);
}

