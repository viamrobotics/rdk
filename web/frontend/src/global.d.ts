/* eslint-disable spaced-comment, multiline-comment-style */
/// <reference types="@types/google.maps" />
/// <reference types="@cypress" />
/// <reference types="vite/client" />
/// <reference types="vue/macros-global" />

import {
  // services
  ArmServiceClient,
  BaseServiceClient,
  BoardServiceClient,
  CameraServiceClient,
  GantryServiceClient,
  GenericServiceClient,
  GripperServiceClient,
  InputControllerServiceClient,
  MotorServiceClient,
  MovementSensorServiceClient,
  ServoServiceClient,
  RobotServiceClient,
  MotionServiceClient,
  NavigationServiceClient,
  SensorsServiceClient,
  SLAMServiceClient,
  VisionServiceClient,
  StreamServiceClient,
  // apis
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
  movementSensorApi,
  servoApi,
  robotApi,
  sensorsApi,
  visionApi,
  streamApi,
} from '@viamrobotics/sdk';

declare global {
  interface Window {
    commonApi: typeof commonApi;
    armApi: typeof armApi;
    baseApi: typeof baseApi;
    boardApi: typeof boardApi;
    cameraApi: typeof cameraApi;
    gantryApi: typeof gantryApi;
    genericApi: typeof genericApi;
    gripperApi: typeof gripperApi;
    inputControllerApi: inputControllerApi;
    motorApi: typeof motorApi;
    movementSensorApi: movementSensorApi;
    robotApi: typeof robotApi;
    sensorsApi: typeof sensorsApi;
    servoApi: typeof servoApi;
    streamApi: typeof streamApi;
    visionApi: typeof visionApi;

    // Service Clients
    streamService: StreamServiceClient;
    robotService: RobotServiceClient;
    armService: ArmServiceClient;
    baseService: BaseServiceClient;
    boardService: BoardServiceClient;
    cameraService: CameraServiceClient;
    gantryService: GantryServiceClient;
    genericService: GenericServiceClient;
    gripperService: GripperServiceClient;
    gpsService: GPSServiceClient;
    inputControllerService: InputControllerServiceClient;
    movementsensorService: MovementSensorServiceClient;
    motorService: MotorServiceClient;
    navigationService: NavigationServiceClient;
    motionService: MotionServiceClient;
    visionService: VisionServiceClient;
    sensorsService: SensorsServiceClient;
    servoService: ServoServiceClient;
    slamService: SLAMServiceClient;

    fetchCameraDiscoveries: import('./lib/discovery').fetchCameraDiscoveries

    // Google
    googleMapsInit: () => void;

    /*
     * Our variables. @TODO: Remove most if not all of these. Do not add more.
     * This is an anti-pattern.
     */
    bakedAuth: {
      authEntity: string;
      creds: import('@viamrobotics/rpc/src/dial').Credentials;
    };
    connect: (
      authEntity?: string,
      creds?: import('@viamrobotics/rpc/src/dial').Credentials
    ) => Promise<void>;
    rcDebug: boolean;
    supportedAuthTypes: string[];
    webrtcAdditionalICEServers: { urls: string; }[];
    webrtcEnabled: boolean;
    webrtcHost: string;
    webrtcSignalingAddress: string;
  }
}

export { };
