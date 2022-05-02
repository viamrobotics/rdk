const { grpc } = require("@improbable-eng/grpc-web");
window.robotApi = require('./gen/proto/api/robot/v1/robot_pb.js');
const { RobotServiceClient } = require('./gen/proto/api/robot/v1/robot_pb_service.js');
window.metadataApi = require('./gen/proto/api/service/metadata/v1/metadata_pb.js');
const { MetadataServiceClient } = require('./gen/proto/api/service/metadata/v1/metadata_pb_service.js');
window.commonApi = require('./gen/proto/api/common/v1/common_pb.js');
window.armApi = require('./gen/proto/api/component/arm/v1/arm_pb.js');
const { ArmServiceClient } = require('./gen/proto/api/component/arm/v1/arm_pb_service.js');
window.baseApi = require('./gen/proto/api/component/base/v1/base_pb.js');
const { BaseServiceClient } = require('./gen/proto/api/component/base/v1/base_pb_service.js');
window.boardApi = require('./gen/proto/api/component/board/v1/board_pb.js');
const { BoardServiceClient } = require('./gen/proto/api/component/board/v1/board_pb_service.js');
window.cameraApi = require('./gen/proto/api/component/camera/v1/camera_pb.js');
const { CameraServiceClient } = require('./gen/proto/api/component/camera/v1/camera_pb_service.js');
window.gantryApi = require('./gen/proto/api/component/gantry/v1/gantry_pb.js');
const { GantryServiceClient } = require('./gen/proto/api/component/gantry/v1/gantry_pb_service.js');
window.gripperApi = require('./gen/proto/api/component/gripper/v1/gripper_pb.js');
const { GripperServiceClient } = require('./gen/proto/api/component/gripper/v1/gripper_pb_service.js');
window.imuApi = require('./gen/proto/api/component/imu/v1/imu_pb.js');
const { IMUServiceClient } = require('./gen/proto/api/component/imu/v1/imu_pb_service.js');
window.inputApi = require('./gen/proto/api/component/inputcontroller/v1/input_controller_pb.js');
const { InputControllerServiceClient } = require('./gen/proto/api/component/inputcontroller/v1/input_controller_pb_service.js');
window.motorApi = require('./gen/proto/api/component/motor/v1/motor_pb.js');
const { MotorServiceClient } = require('./gen/proto/api/component/motor/v1/motor_pb_service.js');
window.navigationApi = require('./gen/proto/api/service/navigation/v1/navigation_pb.js');
const { NavigationServiceClient } = require('./gen/proto/api/service/navigation/v1/navigation_pb_service.js');
window.motionApi = require('./gen/proto/api/service/motion/v1/motion_pb.js');
const { MotionServiceClient } = require('./gen/proto/api/service/motion/v1/motion_pb_service.js');
window.visionApi = require('./gen/proto/api/service/vision/v1/vision_pb.js');
const { VisionServiceClient } = require('./gen/proto/api/service/vision/v1/vision_pb_service.js');
window.sensorsApi = require('./gen/proto/api/service/sensors/v1/sensors_pb.js');
const { SensorsServiceClient } = require('./gen/proto/api/service/sensors/v1/sensors_pb_service.js');
window.servoApi = require('./gen/proto/api/component/servo/v1/servo_pb.js');
const { ServoServiceClient } = require('./gen/proto/api/component/servo/v1/servo_pb_service.js');
window.statusApi = require('./gen/proto/api/service/status/v1/status_pb.js');
const { StatusServiceClient } = require('./gen/proto/api/service/status/v1/status_pb_service.js');
window.streamApi = require("./gen/proto/stream/v1/stream_pb.js");
const { StreamServiceClient } = require('./gen/proto/stream/v1/stream_pb_service.js');
const { dialDirect, dialWebRTC } = require("@viamrobotics/rpc");
window.THREE = require("three")
window.pcdLib = require("../node_modules/three/examples/jsm/loaders/PCDLoader.js")
window.orbitLib = require("../node_modules/three/examples/jsm/controls/OrbitControls.js")
window.trackLib = require("../node_modules/three/examples/jsm/controls/TrackballControls.js")
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

let connect = async (authEntity, creds) => {
	let transportFactory;
	const opts = { 
		authEntity: authEntity,
		credentials: creds,
		webrtcOptions: { rtcConfig: rtcConfig },
	};
	const impliedURL = `${location.protocol}//${location.hostname}${location.port ? ':' + location.port : ''}`;
	if (window.webrtcEnabled) {
		if (!window.webrtcSignalingAddress) {
			window.webrtcSignalingAddress = impliedURL;
		}
		opts.webrtcOptions.signalingAuthEntity = opts.authEntity;
		opts.webrtcOptions.signalingCredentials = opts.credentials;

		const webRTCConn = await dialWebRTC(window.webrtcSignalingAddress, window.webrtcHost, opts);
		transportFactory = webRTCConn.transportFactory
		window.streamService = new StreamServiceClient(window.webrtcHost, { transport: transportFactory });
		webRTCConn.peerConnection.ontrack = async event => {
			const video = document.createElement('video');
			video.srcObject = event.streams[0];
			video.autoplay = true;
			video.controls = false;
			video.playsInline = true;
			const streamName = event.streams[0].id;
			const streamContainer = document.getElementById(`stream-${streamName}`);
			if (streamContainer && streamContainer.getElementsByTagName("video").length > 0) {
				streamContainer.getElementsByTagName("video")[0].remove();
			}
			if (streamContainer) {
				streamContainer.appendChild(video);
			}
			const videoPreview = document.createElement('video');
			videoPreview.srcObject = event.streams[0];
			videoPreview.autoplay = true;
			videoPreview.controls = false;
			videoPreview.playsInline = true;
			const streamPreviewContainer = document.getElementById(`stream-preview-${streamName}`);
			if (streamPreviewContainer && streamPreviewContainer.getElementsByTagName("video").length > 0) {
				streamPreviewContainer.getElementsByTagName("video")[0].remove();
			}
			if (streamPreviewContainer) {
				streamPreviewContainer.appendChild(videoPreview);
			}
		}
	} else {
		transportFactory = await dialDirect(impliedURL, opts);
	}

	// save authEntity, creds
	window.connect = () => connect(authEntity, creds);

	window.robotService = new RobotServiceClient(window.webrtcHost, { transport: transportFactory });
	window.metadataService = new MetadataServiceClient(window.webrtcHost, { transport: transportFactory });

	// TODO: these should be created as needed for #272
	window.armService = new ArmServiceClient(window.webrtcHost, { transport: transportFactory });
	window.baseService = new BaseServiceClient(window.webrtcHost, { transport: transportFactory });
	window.boardService = new BoardServiceClient(window.webrtcHost, { transport: transportFactory });
	window.cameraService = new CameraServiceClient(window.webrtcHost, { transport: transportFactory });
	window.gantryService = new GantryServiceClient(window.webrtcHost, { transport: transportFactory });
	window.gripperService = new GripperServiceClient(window.webrtcHost, { transport: transportFactory });
	window.imuService = new IMUServiceClient(window.webrtcHost, { transport: transportFactory });
	window.inputControllerService = new InputControllerServiceClient(window.webrtcHost, { transport: transportFactory });
	window.motorService = new MotorServiceClient(window.webrtcHost, { transport: transportFactory });
	window.navigationService = new NavigationServiceClient(window.webrtcHost, { transport: transportFactory });
	window.motionService = new MotionServiceClient(window.webrtcHost, { transport: transportFactory });
	window.visionService = new VisionServiceClient(window.webrtcHost, { transport: transportFactory });
	window.sensorsService = new SensorsServiceClient(window.webrtcHost, { transport: transportFactory });
	window.servoService = new ServoServiceClient(window.webrtcHost, { transport: transportFactory });
	window.statusService = new StatusServiceClient(window.webrtcHost, { transport: transportFactory });
}
window.connect = connect;

window.rcDebug = false;
window.rcLogConditionally = function (req) {
	if (rcDebug) {
		console.log("gRPC call: ", req);
	}
}
