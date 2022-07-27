import { dialDirect, dialWebRTC } from '@viamrobotics/rpc';
import { RobotServiceClient } from './gen/proto/api/robot/v1/robot_pb_service.esm';
import { ArmServiceClient } from './gen/proto/api/component/arm/v1/arm_pb_service.esm';
import { BaseServiceClient } from './gen/proto/api/component/base/v1/base_pb_service.esm';
import { BoardServiceClient } from './gen/proto/api/component/board/v1/board_pb_service.esm';
import { CameraServiceClient } from './gen/proto/api/component/camera/v1/camera_pb_service.esm';
import { GantryServiceClient } from './gen/proto/api/component/gantry/v1/gantry_pb_service.esm';
import { GripperServiceClient } from './gen/proto/api/component/gripper/v1/gripper_pb_service.esm';
import { IMUServiceClient } from './gen/proto/api/component/imu/v1/imu_pb_service.esm';
import { InputControllerServiceClient } from './gen/proto/api/component/inputcontroller/v1/input_controller_pb_service.esm';
import { MotorServiceClient } from './gen/proto/api/component/motor/v1/motor_pb_service.esm';
import { NavigationServiceClient } from './gen/proto/api/service/navigation/v1/navigation_pb_service.esm';
import { MotionServiceClient } from './gen/proto/api/service/motion/v1/motion_pb_service.esm';
import { VisionServiceClient } from './gen/proto/api/service/vision/v1/vision_pb_service.esm';
import { SensorsServiceClient } from './gen/proto/api/service/sensors/v1/sensors_pb_service.esm';
import { ServoServiceClient } from './gen/proto/api/component/servo/v1/servo_pb_service.esm';
import { SLAMServiceClient } from './gen/proto/api/service/slam/v1/slam_pb_service.esm';
import { StreamServiceClient } from './gen/proto/stream/v1/stream_pb_service.esm';

import commonApi from './gen/proto/api/common/v1/common_pb.esm';
import armApi from './gen/proto/api/component/arm/v1/arm_pb.esm';
import baseApi from './gen/proto/api/component/base/v1/base_pb.esm';
import cameraApi from './gen/proto/api/component/camera/v1/camera_pb.esm';
import gripperApi from './gen/proto/api/component/gripper/v1/gripper_pb.esm';
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import sensorsApi from './gen/proto/api/service/sensors/v1/sensors_pb.esm';
import servoApi from './gen/proto/api/component/servo/v1/servo_pb.esm';
import streamApi from './gen/proto/stream/v1/stream_pb.esm';

/**
 * Every window variable on this page is being currently used by the blockly page in App.
 * Once we switch blockly to using import / export we should remove / clean up these window variables.
 */
window.commonApi = commonApi;
window.armApi = armApi;
window.baseApi = baseApi;
window.cameraApi = cameraApi;
window.gripperApi = gripperApi;
window.robotApi = robotApi;
window.sensorsApi = sensorsApi;
window.servoApi = servoApi;
window.streamApi = streamApi;

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
      const streamName = event.streams[0].id;
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
