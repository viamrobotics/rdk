/* eslint-disable spaced-comment, multiline-comment-style */
/// <reference types="@types/google.maps" />
/// <reference types="@cypress" />
/// <reference types="vite/client" />
/// <reference types="vue/macros-global" />

declare global {
  interface Window {
    commonApi: typeof import('./gen/proto/api/common/v1/common_pb.esm');
    armApi: typeof import('./gen/proto/api/component/arm/v1/arm_pb.esm');
    baseApi: typeof import('./gen/proto/api/component/base/v1/base_pb.esm');
    boardApi: typeof import('./gen/proto/api/component/board/v1/board_pb.esm');
    cameraApi: typeof import('./gen/proto/api/component/camera/v1/camera_pb.esm');
    gantryApi: typeof import('./gen/proto/api/component/gantry/v1/gantry_pb.esm');
    genericApi: typeof import('./gen/proto/api/component/generic/v1/generic_pb.esm');
    gripperApi: typeof import('./gen/proto/api/component/gripper/v1/gripper_pb.esm');
    inputControllerApi: typeof import('./gen/proto/api/component/inputcontroller/v1/input_controller_pb.esm');
    motorApi: typeof import('./gen/proto/api/component/motor/v1/motor_pb.esm');
    movementSensorApi: typeof import('./gen/proto/api/component/movementsensor/v1/movementsensor_pb.esm');
    robotApi: typeof import('./gen/proto/api/robot/v1/robot_pb.esm');
    sensorsApi: typeof import('./gen/proto/api/service/sensors/v1/sensors_pb.esm');
    servoApi: typeof import('./gen/proto/api/component/servo/v1/servo_pb.esm');
    streamApi: typeof import('./gen/proto/stream/v1/stream_pb.esm');
    visionApi: typeof import('./gen/proto/api/service/vision/v1/vision_pb.esm');

    // Service Clients
    streamService: import('./gen/proto/stream/v1/stream_pb_service.esm').StreamServiceClient;
    robotService: import('./gen/proto/api/robot/v1/robot_pb_service.esm').RobotServiceClient;
    armService: import('./gen/proto/api/component/arm/v1/arm_pb_service.esm').ArmServiceClient;
    baseService: import('./gen/proto/api/component/base/v1/base_pb_service.esm').BaseServiceClient;
    boardService: import('./gen/proto/api/component/board/v1/board_pb_service.esm').BoardServiceClient;
    cameraService: import('./gen/proto/api/component/camera/v1/camera_pb_service.esm').CameraServiceClient;
    gantryService: import('./gen/proto/api/component/gantry/v1/gantry_pb_service.esm').GantryServiceClient;
    genericService: import('./gen/proto/api/component/generic/v1/generic_pb_service.esm').GenericServiceClient;
    gripperService: import('./gen/proto/api/component/gripper/v1/gripper_pb_service.esm').GripperServiceClient;
    gpsService: import('./gen/proto/api/component/gps/v1/gps_pb_service.esm').GPSServiceClient;
    inputControllerService: import('./gen/proto/api/component/inputcontroller/v1/input_controller_pb_service.esm')
    .InputControllerServiceClient;
    movementsensorService: import('./gen/proto/api/component/movementsensor/v1/movementsensor_pb_service.esm')
    .MovementSensorServiceClient;
    motorService: import('./gen/proto/api/component/motor/v1/motor_pb_service.esm').MotorServiceClient;
    navigationService: import('./gen/proto/api/service/navigation/v1/navigation_pb_service.esm')
    .NavigationServiceClient;
    motionService: import('./gen/proto/api/service/motion/v1/motion_pb_service.esm').MotionServiceClient;
    visionService: import('./gen/proto/api/service/vision/v1/vision_pb_service.esm').VisionServiceClient;
    sensorsService: import('./gen/proto/api/service/sensors/v1/sensors_pb_service.esm').SensorsServiceClient;
    servoService: import('./gen/proto/api/component/servo/v1/servo_pb_service.esm').ServoServiceClient;
    slamService: import('./gen/proto/api/service/slam/v1/slam_pb_service.esm').SLAMServiceClient;

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
