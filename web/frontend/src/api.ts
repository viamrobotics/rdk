import { dialDirect, dialWebRTC } from '@viamrobotics/rpc';
import type { Credentials, DialOptions } from '@viamrobotics/rpc/src/dial';
import { ArmServiceClient } from './gen/proto/api/component/arm/v1/arm_pb_service.esm';
import { BaseServiceClient } from './gen/proto/api/component/base/v1/base_pb_service.esm';
import { BoardServiceClient } from './gen/proto/api/component/board/v1/board_pb_service.esm';
import { CameraServiceClient } from './gen/proto/api/component/camera/v1/camera_pb_service.esm';
import { GantryServiceClient } from './gen/proto/api/component/gantry/v1/gantry_pb_service.esm';
import { GenericServiceClient } from './gen/proto/api/component/generic/v1/generic_pb_service.esm';
import { GripperServiceClient } from './gen/proto/api/component/gripper/v1/gripper_pb_service.esm';
import {
  InputControllerServiceClient,
} from './gen/proto/api/component/inputcontroller/v1/input_controller_pb_service.esm';
import { MotorServiceClient } from './gen/proto/api/component/motor/v1/motor_pb_service.esm';
import { MovementSensorServiceClient } from './gen/proto/api/component/movementsensor/v1/movementsensor_pb_service.esm';
import { ServoServiceClient } from './gen/proto/api/component/servo/v1/servo_pb_service.esm';
import { RobotServiceClient } from './gen/proto/api/robot/v1/robot_pb_service.esm';
import { MotionServiceClient } from './gen/proto/api/service/motion/v1/motion_pb_service.esm';
import { NavigationServiceClient } from './gen/proto/api/service/navigation/v1/navigation_pb_service.esm';
import { SensorsServiceClient } from './gen/proto/api/service/sensors/v1/sensors_pb_service.esm';
import { SLAMServiceClient } from './gen/proto/api/service/slam/v1/slam_pb_service.esm';
import { VisionServiceClient } from './gen/proto/api/service/vision/v1/vision_pb_service.esm';
import { StreamServiceClient } from './gen/proto/stream/v1/stream_pb_service.esm';

import commonApi from './gen/proto/api/common/v1/common_pb.esm';
import armApi from './gen/proto/api/component/arm/v1/arm_pb.esm';
import baseApi from './gen/proto/api/component/base/v1/base_pb.esm';
import boardApi from './gen/proto/api/component/board/v1/board_pb.esm';
import cameraApi from './gen/proto/api/component/camera/v1/camera_pb.esm';
import gantryApi from './gen/proto/api/component/gantry/v1/gantry_pb.esm';
import genericApi from './gen/proto/api/component/generic/v1/generic_pb.esm';
import gripperApi from './gen/proto/api/component/gripper/v1/gripper_pb.esm';
import inputControllerApi from './gen/proto/api/component/inputcontroller/v1/input_controller_pb.esm';
import motorApi from './gen/proto/api/component/motor/v1/motor_pb.esm';
import movementSensorApi from './gen/proto/api/component/movementsensor/v1/movementsensor_pb.esm';
import servoApi from './gen/proto/api/component/servo/v1/servo_pb.esm';
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import sensorsApi from './gen/proto/api/service/sensors/v1/sensors_pb.esm';
import visionApi from './gen/proto/api/service/vision/v1/vision_pb.esm';
import streamApi from './gen/proto/stream/v1/stream_pb.esm';

import { fetchCameraDiscoveries } from './lib/discovery';

/**
 * Every window variable on this page is being currently used by the blockly page in App.
 * Once we switch blockly to using import / export we should remove / clean up these window variables.
 */

window.commonApi = commonApi;
window.armApi = armApi;
window.baseApi = baseApi;
window.boardApi = boardApi;
window.cameraApi = cameraApi;
window.gantryApi = gantryApi;
window.genericApi = genericApi;
window.gripperApi = gripperApi;
window.movementSensorApi = movementSensorApi;
window.inputControllerApi = inputControllerApi;
window.motorApi = motorApi;
window.sensorsApi = sensorsApi;
window.servoApi = servoApi;
window.streamApi = streamApi;
window.visionApi = visionApi;

/**
 * This window variable is used by the config page to access the discovery service.
 * As with variables above, once we switch to using import / export we should
 * remove / clean up these window variables.
 */
window.robotApi = robotApi;
window.fetchCameraDiscoveries = fetchCameraDiscoveries;

let savedAuthEntity: string;
let savedCreds: Credentials;

const rtcConfig = {
  iceServers: [
    {
      urls: 'stun:global.stun.twilio.com:3478?transport=udp',
    },
  ],
};

if (window.webrtcAdditionalICEServers) {
  rtcConfig.iceServers = [...rtcConfig.iceServers, ...window.webrtcAdditionalICEServers];
}

let peerConn: RTCPeerConnection | undefined;
let connecting: Promise<void> | undefined;
let connectResolve: (() => void) | undefined;

const connect = async (authEntity = savedAuthEntity, creds = savedCreds) => {
  if (connecting) {
    await connecting;
    return;
  }
  connecting = new Promise<void>((resolve) => {
    connectResolve = resolve;
  });

  if (peerConn) {
    console.log('close old');
    peerConn.close();
    peerConn = undefined;
  }

  try {
    let transportFactory;
    const opts: DialOptions = {
      authEntity,
      credentials: creds,
      webrtcOptions: {
        disableTrickleICE: false,
        rtcConfig,
      },
    };
    const impliedURL = `${location.protocol}//${location.hostname}${location.port ? `:${location.port}` : ''}`;

    // save authEntity, creds
    savedAuthEntity = authEntity;
    savedCreds = creds;

    if (window.webrtcEnabled) {
      opts.webrtcOptions!.signalingAuthEntity = opts.authEntity;
      opts.webrtcOptions!.signalingCredentials = opts.credentials;

      const webRTCConn = await dialWebRTC(window.webrtcSignalingAddress || impliedURL, window.webrtcHost, opts);

      /*
       * lint disabled because we know that we are the only code to
       * read and then write to 'peerConn', even after we have awaited/paused.
       */
      peerConn = webRTCConn.peerConnection; // eslint-disable-line require-atomic-updates
      transportFactory = webRTCConn.transportFactory;

      webRTCConn.peerConnection.ontrack = (event) => {
        const { kind } = event.track;

        const streamName = event.streams[0]!.id;
        const streamContainers = document.querySelectorAll(`[data-stream="${streamName}"]`);

        for (const streamContainer of streamContainers) {
          const mediaElement = document.createElement(kind) as HTMLAudioElement | HTMLVideoElement;
          mediaElement.srcObject = event.streams[0]!;
          mediaElement.autoplay = true;
          if (mediaElement instanceof HTMLVideoElement) {
            mediaElement.playsInline = true;
            mediaElement.controls = false;
          } else {
            mediaElement.controls = true;
          }

          const child = streamContainer.querySelector(kind);
          child?.remove();
          streamContainer.append(mediaElement);
        }

        const streamPreviewContainers = document.querySelectorAll(`[data-stream-preview="${streamName}"]`);
        for (const streamContainer of streamPreviewContainers) {
          const mediaElementPreview = document.createElement(kind) as HTMLAudioElement | HTMLVideoElement;
          mediaElementPreview.srcObject = event.streams[0]!;
          mediaElementPreview.autoplay = true;
          if (mediaElementPreview instanceof HTMLVideoElement) {
            mediaElementPreview.playsInline = true;
            mediaElementPreview.controls = false;
          } else {
            mediaElementPreview.controls = true;
          }
          const child = streamContainer.querySelector(kind);
          child?.remove();
          streamContainer.append(mediaElementPreview);
        }
      };
    } else {
      transportFactory = await dialDirect(impliedURL, opts);
    }

    window.streamService = new StreamServiceClient(window.webrtcHost, { transport: transportFactory });
    window.robotService = new RobotServiceClient(window.webrtcHost, { transport: transportFactory });
    // TODO(RSDK-144): these should be created as needed
    window.armService = new ArmServiceClient(window.webrtcHost, { transport: transportFactory });
    window.baseService = new BaseServiceClient(window.webrtcHost, { transport: transportFactory });
    window.boardService = new BoardServiceClient(window.webrtcHost, { transport: transportFactory });
    window.cameraService = new CameraServiceClient(window.webrtcHost, { transport: transportFactory });
    window.gantryService = new GantryServiceClient(window.webrtcHost, { transport: transportFactory });
    window.genericService = new GenericServiceClient(window.webrtcHost, { transport: transportFactory });
    window.gripperService = new GripperServiceClient(window.webrtcHost, { transport: transportFactory });
    window.movementsensorService = new MovementSensorServiceClient(window.webrtcHost, { transport: transportFactory });
    window.inputControllerService = new InputControllerServiceClient(
      window.webrtcHost, { transport: transportFactory }
    );
    window.motorService = new MotorServiceClient(window.webrtcHost, { transport: transportFactory });
    window.navigationService = new NavigationServiceClient(window.webrtcHost, { transport: transportFactory });
    window.motionService = new MotionServiceClient(window.webrtcHost, { transport: transportFactory });
    window.visionService = new VisionServiceClient(window.webrtcHost, { transport: transportFactory });
    window.sensorsService = new SensorsServiceClient(window.webrtcHost, { transport: transportFactory });
    window.servoService = new ServoServiceClient(window.webrtcHost, { transport: transportFactory });
    window.slamService = new SLAMServiceClient(window.webrtcHost, { transport: transportFactory });
  } finally {
    connectResolve?.();
    connectResolve = undefined;

    /*
     * lint disabled because we know that we are the only code to
     * read and then write to 'connecting', even after we have awaited/paused.
     */
    connecting = undefined; // eslint-disable-line require-atomic-updates
  }
};

window.connect = connect;
