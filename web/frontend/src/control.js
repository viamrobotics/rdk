// eslint-disable-next-line @typescript-eslint/no-unused-vars
const { grpc } = require('@improbable-eng/grpc-web');
window.robotApi = require('./gen/proto/api/robot/v1/robot_pb');
const { RobotServiceClient } = require('./gen/proto/api/robot/v1/robot_pb_service');
window.commonApi = require('./gen/proto/api/common/v1/common_pb');
window.armApi = require('./gen/proto/api/component/arm/v1/arm_pb');
const { ArmServiceClient } = require('./gen/proto/api/component/arm/v1/arm_pb_service');
window.baseApi = require('./gen/proto/api/component/base/v1/base_pb');
const { BaseServiceClient } = require('./gen/proto/api/component/base/v1/base_pb_service');
window.boardApi = require('./gen/proto/api/component/board/v1/board_pb');
const { BoardServiceClient } = require('./gen/proto/api/component/board/v1/board_pb_service');
window.cameraApi = require('./gen/proto/api/component/camera/v1/camera_pb');
const { CameraServiceClient } = require('./gen/proto/api/component/camera/v1/camera_pb_service');
window.gantryApi = require('./gen/proto/api/component/gantry/v1/gantry_pb');
const { GantryServiceClient } = require('./gen/proto/api/component/gantry/v1/gantry_pb_service');
window.gripperApi = require('./gen/proto/api/component/gripper/v1/gripper_pb');
const { GripperServiceClient } = require('./gen/proto/api/component/gripper/v1/gripper_pb_service');
window.imuApi = require('./gen/proto/api/component/imu/v1/imu_pb');
const { IMUServiceClient } = require('./gen/proto/api/component/imu/v1/imu_pb_service');
window.inputApi = require('./gen/proto/api/component/inputcontroller/v1/input_controller_pb');
const { InputControllerServiceClient } = require('./gen/proto/api/component/inputcontroller/v1/input_controller_pb_service');
window.motorApi = require('./gen/proto/api/component/motor/v1/motor_pb');
const { MotorServiceClient } = require('./gen/proto/api/component/motor/v1/motor_pb_service');
window.navigationApi = require('./gen/proto/api/service/navigation/v1/navigation_pb');
const { NavigationServiceClient } = require('./gen/proto/api/service/navigation/v1/navigation_pb_service');
window.motionApi = require('./gen/proto/api/service/motion/v1/motion_pb');
const { MotionServiceClient } = require('./gen/proto/api/service/motion/v1/motion_pb_service');
window.visionApi = require('./gen/proto/api/service/vision/v1/vision_pb');
const { VisionServiceClient } = require('./gen/proto/api/service/vision/v1/vision_pb_service');
window.sensorsApi = require('./gen/proto/api/service/sensors/v1/sensors_pb');
const { SensorsServiceClient } = require('./gen/proto/api/service/sensors/v1/sensors_pb_service');
window.servoApi = require('./gen/proto/api/component/servo/v1/servo_pb');
const { ServoServiceClient } = require('./gen/proto/api/component/servo/v1/servo_pb_service');
window.slamApi = require('./gen/proto/api/service/slam/v1/slam_pb');
const { SLAMServiceClient } = require('./gen/proto/api/service/slam/v1/slam_pb_service');
window.streamApi = require('./gen/proto/stream/v1/stream_pb');
const { StreamServiceClient } = require('./gen/proto/stream/v1/stream_pb_service');
const { dialDirect, dialWebRTC } = require('@viamrobotics/rpc');
window.THREE = require('three');
window.pcdLib = require('../node_modules/three/examples/jsm/loaders/PCDLoader');
window.orbitLib = require('../node_modules/three/examples/jsm/controls/OrbitControls');
window.trackLib = require('../node_modules/three/examples/jsm/controls/TrackballControls');
const rtcConfig = {
	iceServers: [
		{
			urls: 'stun:global.stun.twilio.com:3478?transport=udp',
		},
	],
};

if (window.webrtcAdditionalICEServers) {
	rtcConfig.iceServers = rtcConfig.iceServers.concat(window.webrtcAdditionalICEServers);
}

const connect = async (authEntity, creds) => {
	let transportFactory;
	const opts = { 
		authEntity,
		credentials: creds,
		webrtcOptions: { rtcConfig },
	};
	const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${ location.port}` : ''}`;
	if (window.webrtcEnabled) {
		if (!window.webrtcSignalingAddress) {
			window.webrtcSignalingAddress = impliedURL;
		}
		opts.webrtcOptions.signalingAuthEntity = opts.authEntity;
		opts.webrtcOptions.signalingCredentials = opts.credentials;

		const webRTCConn = await dialWebRTC(window.webrtcSignalingAddress, window.webrtcHost, opts);
		transportFactory = webRTCConn.transportFactory;
		window.streamService = new StreamServiceClient(window.webrtcHost, { transport: transportFactory });
		
		// eslint-disable-next-line require-await
		webRTCConn.peerConnection.ontrack = async event => {
			const video = document.createElement('video');
			video.srcObject = event.streams[0];
			video.autoplay = true;
			video.controls = false;
			video.playsInline = true;
			const streamName = event.streams[0].id;
			const streamContainer = document.getElementById(`stream-${streamName}`);
			if (streamContainer && streamContainer.querySelectorAll('video').length > 0) {
				streamContainer.querySelectorAll('video')[0].remove();
			}
			if (streamContainer) {
				streamContainer.append(video);
			}
			const videoPreview = document.createElement('video');
			videoPreview.srcObject = event.streams[0];
			videoPreview.autoplay = true;
			videoPreview.controls = false;
			videoPreview.playsInline = true;
			const streamPreviewContainer = document.getElementById(`stream-preview-${streamName}`);
			if (streamPreviewContainer && streamPreviewContainer.querySelectorAll('video').length > 0) {
				streamPreviewContainer.querySelectorAll('video')[0].remove();
			}
			if (streamPreviewContainer) {
				streamPreviewContainer.append(videoPreview);
			}
		};
	} else {
		transportFactory = await dialDirect(impliedURL, opts);
	}

	// save authEntity, creds
	window.connect = () => connect(authEntity, creds);

	window.robotService = new RobotServiceClient(window.webrtcHost, { transport: transportFactory });

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
	window.slamService = new SLAMServiceClient(window.webrtcHost, { transport: transportFactory });
};
window.connect = connect;

window.rcDebug = false;
window.rcLogConditionally = function (req) {
	if (rcDebug) {
		console.log('gRPC call:', req);
	}
};
