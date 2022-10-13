/* eslint-disable unicorn/prefer-export-from */
import type { grpc } from '@improbable-eng/grpc-web';
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
import { MotionServiceClient } from './gen/proto/api/service/motion/v1/motion_pb_service.esm';
import { MotorServiceClient } from './gen/proto/api/component/motor/v1/motor_pb_service.esm';
import { MovementSensorServiceClient } from './gen/proto/api/component/movementsensor/v1/movementsensor_pb_service.esm';
import { NavigationServiceClient } from './gen/proto/api/service/navigation/v1/navigation_pb_service.esm';
import { RobotServiceClient } from './gen/proto/api/robot/v1/robot_pb_service.esm';
import { ServoServiceClient } from './gen/proto/api/component/servo/v1/servo_pb_service.esm';
import { SensorsServiceClient } from './gen/proto/api/service/sensors/v1/sensors_pb_service.esm';
import { SLAMServiceClient } from './gen/proto/api/service/slam/v1/slam_pb_service.esm';
import { StreamServiceClient } from './gen/proto/stream/v1/stream_pb_service.esm';
import { VisionServiceClient } from './gen/proto/api/service/vision/v1/vision_pb_service.esm';

export type { ArmServiceClient } from './gen/proto/api/component/arm/v1/arm_pb_service.esm';
export type { BaseServiceClient } from './gen/proto/api/component/base/v1/base_pb_service.esm';
export type { BoardServiceClient } from './gen/proto/api/component/board/v1/board_pb_service.esm';
export type { CameraServiceClient } from './gen/proto/api/component/camera/v1/camera_pb_service.esm';
export type { GantryServiceClient } from './gen/proto/api/component/gantry/v1/gantry_pb_service.esm';
export type { GenericServiceClient } from './gen/proto/api/component/generic/v1/generic_pb_service.esm';
export type { GripperServiceClient } from './gen/proto/api/component/gripper/v1/gripper_pb_service.esm';
export type {
  InputControllerServiceClient
} from './gen/proto/api/component/inputcontroller/v1/input_controller_pb_service.esm';
export type { MotionServiceClient } from './gen/proto/api/service/motion/v1/motion_pb_service.esm';
export type { MotorServiceClient } from './gen/proto/api/component/motor/v1/motor_pb_service.esm';
export type {
  MovementSensorServiceClient
} from './gen/proto/api/component/movementsensor/v1/movementsensor_pb_service.esm';
export type { NavigationServiceClient } from './gen/proto/api/service/navigation/v1/navigation_pb_service.esm';
export type { RobotServiceClient } from './gen/proto/api/robot/v1/robot_pb_service.esm';
export type { ServoServiceClient } from './gen/proto/api/component/servo/v1/servo_pb_service.esm';
export type { SensorsServiceClient } from './gen/proto/api/service/sensors/v1/sensors_pb_service.esm';
export type { SLAMServiceClient } from './gen/proto/api/service/slam/v1/slam_pb_service.esm';
export type { StreamServiceClient } from './gen/proto/stream/v1/stream_pb_service.esm';
export type { VisionServiceClient } from './gen/proto/api/service/vision/v1/vision_pb_service.esm';

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
import motionApi from './gen/proto/api/service/motion/v1/motion_pb.esm';
import movementSensorApi from './gen/proto/api/component/movementsensor/v1/movementsensor_pb.esm';
import navigationApi from './gen/proto/api/service/navigation/v1/navigation_pb.esm';
import robotApi from './gen/proto/api/robot/v1/robot_pb.esm';
import sensorsApi from './gen/proto/api/service/sensors/v1/sensors_pb.esm';
import servoApi from './gen/proto/api/component/servo/v1/servo_pb.esm';
import slamApi from './gen/proto/api/service/slam/v1/slam_pb.esm';
import streamApi from './gen/proto/stream/v1/stream_pb.esm';
import visionApi from './gen/proto/api/service/vision/v1/vision_pb.esm';

export {
  commonApi,
  armApi,
  baseApi,
  boardApi,
  cameraApi,
  gantryApi,
  genericApi,
  gripperApi,
  inputControllerApi,
  motorApi,
  motionApi,
  movementSensorApi,
  navigationApi,
  robotApi,
  sensorsApi,
  servoApi,
  slamApi,
  streamApi,
  visionApi
};

let transport: grpc.TransportFactory;

export const createStreamService = () => new StreamServiceClient(window.webrtcHost, { transport });
export const createRobotService = () => new RobotServiceClient(window.webrtcHost, { transport });
export const createArmService = () => new ArmServiceClient(window.webrtcHost, { transport });
export const createBaseService = () => new BaseServiceClient(window.webrtcHost, { transport });
export const createBoardService = () => new BoardServiceClient(window.webrtcHost, { transport });
export const createCameraService = () => new CameraServiceClient(window.webrtcHost, { transport });
export const createGantryService = () => new GantryServiceClient(window.webrtcHost, { transport });
export const createGenericService = () => new GenericServiceClient(window.webrtcHost, { transport });
export const createGripperService = () => new GripperServiceClient(window.webrtcHost, { transport });
export const createMovementSensorService = () => new MovementSensorServiceClient(window.webrtcHost, { transport });
export const createInputControllerService = () => new InputControllerServiceClient(window.webrtcHost, { transport });
export const createMotorService = () => new MotorServiceClient(window.webrtcHost, { transport });
export const createNavigationService = () => new NavigationServiceClient(window.webrtcHost, { transport });
export const createMotionService = () => new MotionServiceClient(window.webrtcHost, { transport });
export const createVisionService = () => new VisionServiceClient(window.webrtcHost, { transport });
export const createSensorsService = () => new SensorsServiceClient(window.webrtcHost, { transport });
export const createServoService = () => new ServoServiceClient(window.webrtcHost, { transport });
export const createSlamService = () => new SLAMServiceClient(window.webrtcHost, { transport });

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

const connect = async (authEntity = savedAuthEntity, creds = savedCreds) => {
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
    transport = webRTCConn.transportFactory;

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
    transport = await dialDirect(impliedURL, opts);
  }
};

window.connect = connect;
