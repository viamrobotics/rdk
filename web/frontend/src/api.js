const { dialDirect, dialWebRTC } = require('@viamrobotics/rpc');
const { normalizeRemoteName } = require('./lib/resource');

const commonApi = require('./gen/proto/api/common/v2/common_pb.esm');
const armApi = require('./gen/proto/api/component/arm/v1/arm_pb.esm');
const baseApi = require('./gen/proto/api/component/base/v1/base_pb.esm');
const cameraApi = require('./gen/proto/api/component/camera/v1/camera_pb.esm');
const gripperApi = require('./gen/proto/api/component/gripper/v1/gripper_pb.esm');
const robotApi = require('./gen/proto/api/robot/v1/robot_pb.esm');
const sensorsApi = require('./gen/proto/api/service/sensors/v1/sensors_pb.esm');
const servoApi = require('./gen/proto/api/component/servo/v1/servo_pb.esm');
const streamApi = require('./gen/proto/stream/v1/stream_pb.esm');
const motorApi = require('./gen/proto/api/component/motor/v1/motor_pb.esm');

const { RobotServiceClient } = require('./gen/proto/api/robot/v1/robot_pb_service.esm');
const { ArmServiceClient } = require('./gen/proto/api/component/arm/v1/arm_pb_service.esm');
const { BaseServiceClient } = require('./gen/proto/api/component/base/v1/base_pb_service.esm');
const { BoardServiceClient } = require('./gen/proto/api/component/board/v1/board_pb_service.esm');
const { CameraServiceClient } = require('./gen/proto/api/component/camera/v1/camera_pb_service.esm');
const { GantryServiceClient } = require('./gen/proto/api/component/gantry/v1/gantry_pb_service.esm');
const { GripperServiceClient } = require('./gen/proto/api/component/gripper/v1/gripper_pb_service.esm');
const { IMUServiceClient } = require('./gen/proto/api/component/imu/v1/imu_pb_service.esm');
const { InputControllerServiceClient } = require('./gen/proto/api/component/inputcontroller/v1/input_controller_pb_service.esm');
const { MotorServiceClient } = require('./gen/proto/api/component/motor/v1/motor_pb_service.esm');
const { NavigationServiceClient } = require('./gen/proto/api/service/navigation/v1/navigation_pb_service.esm');
const { MotionServiceClient } = require('./gen/proto/api/service/motion/v1/motion_pb_service.esm');
const { VisionServiceClient } = require('./gen/proto/api/service/vision/v1/vision_pb_service.esm');
const { SensorsServiceClient } = require('./gen/proto/api/service/sensors/v1/sensors_pb_service.esm');
const { ServoServiceClient } = require('./gen/proto/api/component/servo/v1/servo_pb_service.esm');
const { SLAMServiceClient } = require('./gen/proto/api/service/slam/v1/slam_pb_service.esm');
const { StreamServiceClient } = require('./gen/proto/stream/v1/stream_pb_service.esm');

/**
 * Every window variable on this page is being currently used by the blockly page in App.
 * Once we switch blockly to using import / export we should remove / clean up these window variables.
 */
window.commonApi = commonApi;
window.armApi = armApi;
window.baseApi = baseApi;
window.cameraApi = cameraApi;
window.gripperApi = gripperApi;
window.sensorsApi = sensorsApi;
window.servoApi = servoApi;
window.streamApi = streamApi;
window.motorApi = motorApi;
/**
 * This window variable is used by the config page to access the discovery service.
 * As with variables above, once we switch to using import / export we should
 * remove / clean up these window variables.
 */
window.robotApi = robotApi;

let savedAuthEntity;
let savedCreds;

const {
  webrtcEnabled,
  webrtcHost,
  webrtcAdditionalICEServers,
  webrtcSignalingAddress,
} = window;

const rtcConfig = {
  iceServers: [
    {
      urls: 'stun:global.stun.twilio.com:3478?transport=udp',
    },
  ],
};

if (webrtcAdditionalICEServers) {
  rtcConfig.iceServers = [...rtcConfig.iceServers, ...webrtcAdditionalICEServers];
}

const connect = async (authEntity = savedAuthEntity, creds = savedCreds) => {
  let transportFactory;
  const opts = { 
    authEntity,
    credentials: creds,
    webrtcOptions: { rtcConfig },
  };
  const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;

  // save authEntity, creds
  savedAuthEntity = authEntity;
  savedCreds = creds;
  
  if (webrtcEnabled) {
    opts.webrtcOptions.signalingAuthEntity = opts.authEntity;
    opts.webrtcOptions.signalingCredentials = opts.credentials;

    const webRTCConn = await dialWebRTC(webrtcSignalingAddress || impliedURL, webrtcHost, opts);
    transportFactory = webRTCConn.transportFactory;

    webRTCConn.peerConnection.ontrack = (event) => {
      const video = document.createElement('video');
      video.srcObject = event.streams[0];
      video.autoplay = true;
      video.controls = false;
      video.playsInline = true;
      let streamName = event.streams[0].id;
      streamName = normalizeRemoteName(streamName);
      const streamContainer = document.querySelector(`#stream-${streamName}`);
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
      const streamPreviewContainer = document.querySelector(`#stream-preview-${streamName}`);
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

  window.streamService = new StreamServiceClient(webrtcHost, { transport: transportFactory });
  window.robotService = new RobotServiceClient(webrtcHost, { transport: transportFactory });
  // TODO(RSDK-144): these should be created as needed
  window.armService = new ArmServiceClient(webrtcHost, { transport: transportFactory });
  window.baseService = new BaseServiceClient(webrtcHost, { transport: transportFactory });
  window.boardService = new BoardServiceClient(webrtcHost, { transport: transportFactory });
  window.cameraService = new CameraServiceClient(webrtcHost, { transport: transportFactory });
  window.gantryService = new GantryServiceClient(webrtcHost, { transport: transportFactory });
  window.gripperService = new GripperServiceClient(webrtcHost, { transport: transportFactory });
  window.imuService = new IMUServiceClient(webrtcHost, { transport: transportFactory });
  window.inputControllerService = new InputControllerServiceClient(webrtcHost, { transport: transportFactory });
  window.motorService = new MotorServiceClient(webrtcHost, { transport: transportFactory });
  window.navigationService = new NavigationServiceClient(webrtcHost, { transport: transportFactory });
  window.motionService = new MotionServiceClient(webrtcHost, { transport: transportFactory });
  window.visionService = new VisionServiceClient(webrtcHost, { transport: transportFactory });
  window.sensorsService = new SensorsServiceClient(webrtcHost, { transport: transportFactory });
  window.servoService = new ServoServiceClient(webrtcHost, { transport: transportFactory });
  window.slamService = new SLAMServiceClient(webrtcHost, { transport: transportFactory });
};

window.connect = connect;
