// package: proto.api.v1
// file: proto/api/v1/robot.proto

import * as proto_api_v1_robot_pb from "../../../proto/api/v1/robot_pb";
import * as google_api_httpbody_pb from "../../../google/api/httpbody_pb";
import {grpc} from "@improbable-eng/grpc-web";

type RobotServiceStatus = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.StatusRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.StatusResponse;
};

type RobotServiceStatusStream = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof proto_api_v1_robot_pb.StatusStreamRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.StatusStreamResponse;
};

type RobotServiceConfig = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ConfigRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ConfigResponse;
};

type RobotServiceDoAction = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.DoActionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.DoActionResponse;
};

type RobotServiceBaseMoveStraight = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BaseMoveStraightRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BaseMoveStraightResponse;
};

type RobotServiceBaseMoveArc = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BaseMoveArcRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BaseMoveArcResponse;
};

type RobotServiceBaseSpin = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BaseSpinRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BaseSpinResponse;
};

type RobotServiceBaseStop = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BaseStopRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BaseStopResponse;
};

type RobotServiceBaseWidthMillis = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BaseWidthMillisRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BaseWidthMillisResponse;
};

type RobotServiceGripperOpen = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.GripperOpenRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.GripperOpenResponse;
};

type RobotServiceGripperGrab = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.GripperGrabRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.GripperGrabResponse;
};

type RobotServiceCameraFrame = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.CameraFrameRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.CameraFrameResponse;
};

type RobotServiceCameraRenderFrame = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.CameraRenderFrameRequest;
  readonly responseType: typeof google_api_httpbody_pb.HttpBody;
};

type RobotServicePointCloud = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.PointCloudRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.PointCloudResponse;
};

type RobotServiceObjectPointClouds = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ObjectPointCloudsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ObjectPointCloudsResponse;
};

type RobotServiceLidarInfo = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.LidarInfoRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.LidarInfoResponse;
};

type RobotServiceLidarStart = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.LidarStartRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.LidarStartResponse;
};

type RobotServiceLidarStop = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.LidarStopRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.LidarStopResponse;
};

type RobotServiceLidarScan = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.LidarScanRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.LidarScanResponse;
};

type RobotServiceLidarRange = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.LidarRangeRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.LidarRangeResponse;
};

type RobotServiceLidarBounds = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.LidarBoundsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.LidarBoundsResponse;
};

type RobotServiceLidarAngularResolution = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.LidarAngularResolutionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.LidarAngularResolutionResponse;
};

type RobotServiceBoardStatus = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardStatusRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardStatusResponse;
};

type RobotServiceBoardGPIOSet = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardGPIOSetRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardGPIOSetResponse;
};

type RobotServiceBoardGPIOGet = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardGPIOGetRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardGPIOGetResponse;
};

type RobotServiceBoardPWMSet = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardPWMSetRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardPWMSetResponse;
};

type RobotServiceBoardPWMSetFrequency = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardPWMSetFrequencyRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardPWMSetFrequencyResponse;
};

type RobotServiceBoardAnalogReaderRead = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardAnalogReaderReadRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardAnalogReaderReadResponse;
};

type RobotServiceBoardDigitalInterruptConfig = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardDigitalInterruptConfigRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardDigitalInterruptConfigResponse;
};

type RobotServiceBoardDigitalInterruptValue = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardDigitalInterruptValueRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardDigitalInterruptValueResponse;
};

type RobotServiceBoardDigitalInterruptTick = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardDigitalInterruptTickRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardDigitalInterruptTickResponse;
};

type RobotServiceSensorReadings = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.SensorReadingsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.SensorReadingsResponse;
};

type RobotServiceCompassHeading = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.CompassHeadingRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.CompassHeadingResponse;
};

type RobotServiceCompassStartCalibration = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.CompassStartCalibrationRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.CompassStartCalibrationResponse;
};

type RobotServiceCompassStopCalibration = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.CompassStopCalibrationRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.CompassStopCalibrationResponse;
};

type RobotServiceCompassMark = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.CompassMarkRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.CompassMarkResponse;
};

type RobotServiceForceMatrixMatrix = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ForceMatrixMatrixRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ForceMatrixMatrixResponse;
};

type RobotServiceForceMatrixSlipDetection = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ForceMatrixSlipDetectionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ForceMatrixSlipDetectionResponse;
};

type RobotServiceExecuteFunction = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ExecuteFunctionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ExecuteFunctionResponse;
};

type RobotServiceExecuteSource = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ExecuteSourceRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ExecuteSourceResponse;
};

type RobotServiceServoMove = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ServoMoveRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ServoMoveResponse;
};

type RobotServiceServoCurrent = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ServoCurrentRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ServoCurrentResponse;
};

type RobotServiceMotorGetPIDConfig = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorGetPIDConfigRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorGetPIDConfigResponse;
};

type RobotServiceMotorSetPIDConfig = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorSetPIDConfigRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorSetPIDConfigResponse;
};

type RobotServiceMotorPIDStep = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorPIDStepRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorPIDStepResponse;
};

type RobotServiceMotorPower = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorPowerRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorPowerResponse;
};

type RobotServiceMotorGo = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorGoRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorGoResponse;
};

type RobotServiceMotorGoFor = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorGoForRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorGoForResponse;
};

type RobotServiceMotorGoTo = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorGoToRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorGoToResponse;
};

type RobotServiceMotorGoTillStop = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorGoTillStopRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorGoTillStopResponse;
};

type RobotServiceMotorZero = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorZeroRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorZeroResponse;
};

type RobotServiceMotorPosition = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorPositionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorPositionResponse;
};

type RobotServiceMotorPositionSupported = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorPositionSupportedRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorPositionSupportedResponse;
};

type RobotServiceMotorOff = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorOffRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorOffResponse;
};

type RobotServiceMotorIsOn = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.MotorIsOnRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.MotorIsOnResponse;
};

type RobotServiceInputControllerControls = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.InputControllerControlsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.InputControllerControlsResponse;
};

type RobotServiceInputControllerLastEvents = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.InputControllerLastEventsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.InputControllerLastEventsResponse;
};

type RobotServiceInputControllerEventStream = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof proto_api_v1_robot_pb.InputControllerEventStreamRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.InputControllerEvent;
};

type RobotServiceInputControllerInjectEvent = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.InputControllerInjectEventRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.InputControllerInjectEventResponse;
};

type RobotServiceResourceRunCommand = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ResourceRunCommandRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ResourceRunCommandResponse;
};

type RobotServiceFrameServiceConfig = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.FrameServiceConfigRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.FrameServiceConfigResponse;
};

type RobotServiceNavigationServiceMode = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.NavigationServiceModeRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.NavigationServiceModeResponse;
};

type RobotServiceNavigationServiceSetMode = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.NavigationServiceSetModeRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.NavigationServiceSetModeResponse;
};

type RobotServiceNavigationServiceLocation = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.NavigationServiceLocationRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.NavigationServiceLocationResponse;
};

type RobotServiceNavigationServiceWaypoints = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.NavigationServiceWaypointsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.NavigationServiceWaypointsResponse;
};

type RobotServiceNavigationServiceAddWaypoint = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.NavigationServiceAddWaypointRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.NavigationServiceAddWaypointResponse;
};

type RobotServiceNavigationServiceRemoveWaypoint = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.NavigationServiceRemoveWaypointRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.NavigationServiceRemoveWaypointResponse;
};

type RobotServiceObjectManipulationServiceDoGrab = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabResponse;
};

type RobotServiceIMUAngularVelocity = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.IMUAngularVelocityRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.IMUAngularVelocityResponse;
};

type RobotServiceIMUOrientation = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.IMUOrientationRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.IMUOrientationResponse;
};

type RobotServiceGPSLocation = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.GPSLocationRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.GPSLocationResponse;
};

type RobotServiceGPSAltitude = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.GPSAltitudeRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.GPSAltitudeResponse;
};

type RobotServiceGPSSpeed = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.GPSSpeedRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.GPSSpeedResponse;
};

type RobotServiceGPSAccuracy = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.GPSAccuracyRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.GPSAccuracyResponse;
};

export class RobotService {
  static readonly serviceName: string;
  static readonly Status: RobotServiceStatus;
  static readonly StatusStream: RobotServiceStatusStream;
  static readonly Config: RobotServiceConfig;
  static readonly DoAction: RobotServiceDoAction;
  static readonly BaseMoveStraight: RobotServiceBaseMoveStraight;
  static readonly BaseMoveArc: RobotServiceBaseMoveArc;
  static readonly BaseSpin: RobotServiceBaseSpin;
  static readonly BaseStop: RobotServiceBaseStop;
  static readonly BaseWidthMillis: RobotServiceBaseWidthMillis;
  static readonly GripperOpen: RobotServiceGripperOpen;
  static readonly GripperGrab: RobotServiceGripperGrab;
  static readonly CameraFrame: RobotServiceCameraFrame;
  static readonly CameraRenderFrame: RobotServiceCameraRenderFrame;
  static readonly PointCloud: RobotServicePointCloud;
  static readonly ObjectPointClouds: RobotServiceObjectPointClouds;
  static readonly LidarInfo: RobotServiceLidarInfo;
  static readonly LidarStart: RobotServiceLidarStart;
  static readonly LidarStop: RobotServiceLidarStop;
  static readonly LidarScan: RobotServiceLidarScan;
  static readonly LidarRange: RobotServiceLidarRange;
  static readonly LidarBounds: RobotServiceLidarBounds;
  static readonly LidarAngularResolution: RobotServiceLidarAngularResolution;
  static readonly BoardStatus: RobotServiceBoardStatus;
  static readonly BoardGPIOSet: RobotServiceBoardGPIOSet;
  static readonly BoardGPIOGet: RobotServiceBoardGPIOGet;
  static readonly BoardPWMSet: RobotServiceBoardPWMSet;
  static readonly BoardPWMSetFrequency: RobotServiceBoardPWMSetFrequency;
  static readonly BoardAnalogReaderRead: RobotServiceBoardAnalogReaderRead;
  static readonly BoardDigitalInterruptConfig: RobotServiceBoardDigitalInterruptConfig;
  static readonly BoardDigitalInterruptValue: RobotServiceBoardDigitalInterruptValue;
  static readonly BoardDigitalInterruptTick: RobotServiceBoardDigitalInterruptTick;
  static readonly SensorReadings: RobotServiceSensorReadings;
  static readonly CompassHeading: RobotServiceCompassHeading;
  static readonly CompassStartCalibration: RobotServiceCompassStartCalibration;
  static readonly CompassStopCalibration: RobotServiceCompassStopCalibration;
  static readonly CompassMark: RobotServiceCompassMark;
  static readonly ForceMatrixMatrix: RobotServiceForceMatrixMatrix;
  static readonly ForceMatrixSlipDetection: RobotServiceForceMatrixSlipDetection;
  static readonly ExecuteFunction: RobotServiceExecuteFunction;
  static readonly ExecuteSource: RobotServiceExecuteSource;
  static readonly ServoMove: RobotServiceServoMove;
  static readonly ServoCurrent: RobotServiceServoCurrent;
  static readonly MotorGetPIDConfig: RobotServiceMotorGetPIDConfig;
  static readonly MotorSetPIDConfig: RobotServiceMotorSetPIDConfig;
  static readonly MotorPIDStep: RobotServiceMotorPIDStep;
  static readonly MotorPower: RobotServiceMotorPower;
  static readonly MotorGo: RobotServiceMotorGo;
  static readonly MotorGoFor: RobotServiceMotorGoFor;
  static readonly MotorGoTo: RobotServiceMotorGoTo;
  static readonly MotorGoTillStop: RobotServiceMotorGoTillStop;
  static readonly MotorZero: RobotServiceMotorZero;
  static readonly MotorPosition: RobotServiceMotorPosition;
  static readonly MotorPositionSupported: RobotServiceMotorPositionSupported;
  static readonly MotorOff: RobotServiceMotorOff;
  static readonly MotorIsOn: RobotServiceMotorIsOn;
  static readonly InputControllerControls: RobotServiceInputControllerControls;
  static readonly InputControllerLastEvents: RobotServiceInputControllerLastEvents;
  static readonly InputControllerEventStream: RobotServiceInputControllerEventStream;
  static readonly InputControllerInjectEvent: RobotServiceInputControllerInjectEvent;
  static readonly ResourceRunCommand: RobotServiceResourceRunCommand;
  static readonly FrameServiceConfig: RobotServiceFrameServiceConfig;
  static readonly NavigationServiceMode: RobotServiceNavigationServiceMode;
  static readonly NavigationServiceSetMode: RobotServiceNavigationServiceSetMode;
  static readonly NavigationServiceLocation: RobotServiceNavigationServiceLocation;
  static readonly NavigationServiceWaypoints: RobotServiceNavigationServiceWaypoints;
  static readonly NavigationServiceAddWaypoint: RobotServiceNavigationServiceAddWaypoint;
  static readonly NavigationServiceRemoveWaypoint: RobotServiceNavigationServiceRemoveWaypoint;
  static readonly ObjectManipulationServiceDoGrab: RobotServiceObjectManipulationServiceDoGrab;
  static readonly IMUAngularVelocity: RobotServiceIMUAngularVelocity;
  static readonly IMUOrientation: RobotServiceIMUOrientation;
  static readonly GPSLocation: RobotServiceGPSLocation;
  static readonly GPSAltitude: RobotServiceGPSAltitude;
  static readonly GPSSpeed: RobotServiceGPSSpeed;
  static readonly GPSAccuracy: RobotServiceGPSAccuracy;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(type: 'data', handler: (message: ResT) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'end', handler: (status?: Status) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'status', handler: (status: Status) => void): BidirectionalStream<ReqT, ResT>;
}

export class RobotServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  status(
    requestMessage: proto_api_v1_robot_pb.StatusRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.StatusResponse|null) => void
  ): UnaryResponse;
  status(
    requestMessage: proto_api_v1_robot_pb.StatusRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.StatusResponse|null) => void
  ): UnaryResponse;
  statusStream(requestMessage: proto_api_v1_robot_pb.StatusStreamRequest, metadata?: grpc.Metadata): ResponseStream<proto_api_v1_robot_pb.StatusStreamResponse>;
  config(
    requestMessage: proto_api_v1_robot_pb.ConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ConfigResponse|null) => void
  ): UnaryResponse;
  config(
    requestMessage: proto_api_v1_robot_pb.ConfigRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ConfigResponse|null) => void
  ): UnaryResponse;
  doAction(
    requestMessage: proto_api_v1_robot_pb.DoActionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.DoActionResponse|null) => void
  ): UnaryResponse;
  doAction(
    requestMessage: proto_api_v1_robot_pb.DoActionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.DoActionResponse|null) => void
  ): UnaryResponse;
  baseMoveStraight(
    requestMessage: proto_api_v1_robot_pb.BaseMoveStraightRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseMoveStraightResponse|null) => void
  ): UnaryResponse;
  baseMoveStraight(
    requestMessage: proto_api_v1_robot_pb.BaseMoveStraightRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseMoveStraightResponse|null) => void
  ): UnaryResponse;
  baseMoveArc(
    requestMessage: proto_api_v1_robot_pb.BaseMoveArcRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseMoveArcResponse|null) => void
  ): UnaryResponse;
  baseMoveArc(
    requestMessage: proto_api_v1_robot_pb.BaseMoveArcRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseMoveArcResponse|null) => void
  ): UnaryResponse;
  baseSpin(
    requestMessage: proto_api_v1_robot_pb.BaseSpinRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseSpinResponse|null) => void
  ): UnaryResponse;
  baseSpin(
    requestMessage: proto_api_v1_robot_pb.BaseSpinRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseSpinResponse|null) => void
  ): UnaryResponse;
  baseStop(
    requestMessage: proto_api_v1_robot_pb.BaseStopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseStopResponse|null) => void
  ): UnaryResponse;
  baseStop(
    requestMessage: proto_api_v1_robot_pb.BaseStopRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseStopResponse|null) => void
  ): UnaryResponse;
  baseWidthMillis(
    requestMessage: proto_api_v1_robot_pb.BaseWidthMillisRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseWidthMillisResponse|null) => void
  ): UnaryResponse;
  baseWidthMillis(
    requestMessage: proto_api_v1_robot_pb.BaseWidthMillisRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BaseWidthMillisResponse|null) => void
  ): UnaryResponse;
  gripperOpen(
    requestMessage: proto_api_v1_robot_pb.GripperOpenRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GripperOpenResponse|null) => void
  ): UnaryResponse;
  gripperOpen(
    requestMessage: proto_api_v1_robot_pb.GripperOpenRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GripperOpenResponse|null) => void
  ): UnaryResponse;
  gripperGrab(
    requestMessage: proto_api_v1_robot_pb.GripperGrabRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GripperGrabResponse|null) => void
  ): UnaryResponse;
  gripperGrab(
    requestMessage: proto_api_v1_robot_pb.GripperGrabRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GripperGrabResponse|null) => void
  ): UnaryResponse;
  cameraFrame(
    requestMessage: proto_api_v1_robot_pb.CameraFrameRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CameraFrameResponse|null) => void
  ): UnaryResponse;
  cameraFrame(
    requestMessage: proto_api_v1_robot_pb.CameraFrameRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CameraFrameResponse|null) => void
  ): UnaryResponse;
  cameraRenderFrame(
    requestMessage: proto_api_v1_robot_pb.CameraRenderFrameRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_httpbody_pb.HttpBody|null) => void
  ): UnaryResponse;
  cameraRenderFrame(
    requestMessage: proto_api_v1_robot_pb.CameraRenderFrameRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_httpbody_pb.HttpBody|null) => void
  ): UnaryResponse;
  pointCloud(
    requestMessage: proto_api_v1_robot_pb.PointCloudRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.PointCloudResponse|null) => void
  ): UnaryResponse;
  pointCloud(
    requestMessage: proto_api_v1_robot_pb.PointCloudRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.PointCloudResponse|null) => void
  ): UnaryResponse;
  objectPointClouds(
    requestMessage: proto_api_v1_robot_pb.ObjectPointCloudsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ObjectPointCloudsResponse|null) => void
  ): UnaryResponse;
  objectPointClouds(
    requestMessage: proto_api_v1_robot_pb.ObjectPointCloudsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ObjectPointCloudsResponse|null) => void
  ): UnaryResponse;
  lidarInfo(
    requestMessage: proto_api_v1_robot_pb.LidarInfoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarInfoResponse|null) => void
  ): UnaryResponse;
  lidarInfo(
    requestMessage: proto_api_v1_robot_pb.LidarInfoRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarInfoResponse|null) => void
  ): UnaryResponse;
  lidarStart(
    requestMessage: proto_api_v1_robot_pb.LidarStartRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarStartResponse|null) => void
  ): UnaryResponse;
  lidarStart(
    requestMessage: proto_api_v1_robot_pb.LidarStartRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarStartResponse|null) => void
  ): UnaryResponse;
  lidarStop(
    requestMessage: proto_api_v1_robot_pb.LidarStopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarStopResponse|null) => void
  ): UnaryResponse;
  lidarStop(
    requestMessage: proto_api_v1_robot_pb.LidarStopRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarStopResponse|null) => void
  ): UnaryResponse;
  lidarScan(
    requestMessage: proto_api_v1_robot_pb.LidarScanRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarScanResponse|null) => void
  ): UnaryResponse;
  lidarScan(
    requestMessage: proto_api_v1_robot_pb.LidarScanRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarScanResponse|null) => void
  ): UnaryResponse;
  lidarRange(
    requestMessage: proto_api_v1_robot_pb.LidarRangeRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarRangeResponse|null) => void
  ): UnaryResponse;
  lidarRange(
    requestMessage: proto_api_v1_robot_pb.LidarRangeRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarRangeResponse|null) => void
  ): UnaryResponse;
  lidarBounds(
    requestMessage: proto_api_v1_robot_pb.LidarBoundsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarBoundsResponse|null) => void
  ): UnaryResponse;
  lidarBounds(
    requestMessage: proto_api_v1_robot_pb.LidarBoundsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarBoundsResponse|null) => void
  ): UnaryResponse;
  lidarAngularResolution(
    requestMessage: proto_api_v1_robot_pb.LidarAngularResolutionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarAngularResolutionResponse|null) => void
  ): UnaryResponse;
  lidarAngularResolution(
    requestMessage: proto_api_v1_robot_pb.LidarAngularResolutionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.LidarAngularResolutionResponse|null) => void
  ): UnaryResponse;
  boardStatus(
    requestMessage: proto_api_v1_robot_pb.BoardStatusRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardStatusResponse|null) => void
  ): UnaryResponse;
  boardStatus(
    requestMessage: proto_api_v1_robot_pb.BoardStatusRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardStatusResponse|null) => void
  ): UnaryResponse;
  boardGPIOSet(
    requestMessage: proto_api_v1_robot_pb.BoardGPIOSetRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardGPIOSetResponse|null) => void
  ): UnaryResponse;
  boardGPIOSet(
    requestMessage: proto_api_v1_robot_pb.BoardGPIOSetRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardGPIOSetResponse|null) => void
  ): UnaryResponse;
  boardGPIOGet(
    requestMessage: proto_api_v1_robot_pb.BoardGPIOGetRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardGPIOGetResponse|null) => void
  ): UnaryResponse;
  boardGPIOGet(
    requestMessage: proto_api_v1_robot_pb.BoardGPIOGetRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardGPIOGetResponse|null) => void
  ): UnaryResponse;
  boardPWMSet(
    requestMessage: proto_api_v1_robot_pb.BoardPWMSetRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardPWMSetResponse|null) => void
  ): UnaryResponse;
  boardPWMSet(
    requestMessage: proto_api_v1_robot_pb.BoardPWMSetRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardPWMSetResponse|null) => void
  ): UnaryResponse;
  boardPWMSetFrequency(
    requestMessage: proto_api_v1_robot_pb.BoardPWMSetFrequencyRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardPWMSetFrequencyResponse|null) => void
  ): UnaryResponse;
  boardPWMSetFrequency(
    requestMessage: proto_api_v1_robot_pb.BoardPWMSetFrequencyRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardPWMSetFrequencyResponse|null) => void
  ): UnaryResponse;
  boardAnalogReaderRead(
    requestMessage: proto_api_v1_robot_pb.BoardAnalogReaderReadRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardAnalogReaderReadResponse|null) => void
  ): UnaryResponse;
  boardAnalogReaderRead(
    requestMessage: proto_api_v1_robot_pb.BoardAnalogReaderReadRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardAnalogReaderReadResponse|null) => void
  ): UnaryResponse;
  boardDigitalInterruptConfig(
    requestMessage: proto_api_v1_robot_pb.BoardDigitalInterruptConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardDigitalInterruptConfigResponse|null) => void
  ): UnaryResponse;
  boardDigitalInterruptConfig(
    requestMessage: proto_api_v1_robot_pb.BoardDigitalInterruptConfigRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardDigitalInterruptConfigResponse|null) => void
  ): UnaryResponse;
  boardDigitalInterruptValue(
    requestMessage: proto_api_v1_robot_pb.BoardDigitalInterruptValueRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardDigitalInterruptValueResponse|null) => void
  ): UnaryResponse;
  boardDigitalInterruptValue(
    requestMessage: proto_api_v1_robot_pb.BoardDigitalInterruptValueRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardDigitalInterruptValueResponse|null) => void
  ): UnaryResponse;
  boardDigitalInterruptTick(
    requestMessage: proto_api_v1_robot_pb.BoardDigitalInterruptTickRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardDigitalInterruptTickResponse|null) => void
  ): UnaryResponse;
  boardDigitalInterruptTick(
    requestMessage: proto_api_v1_robot_pb.BoardDigitalInterruptTickRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardDigitalInterruptTickResponse|null) => void
  ): UnaryResponse;
  sensorReadings(
    requestMessage: proto_api_v1_robot_pb.SensorReadingsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.SensorReadingsResponse|null) => void
  ): UnaryResponse;
  sensorReadings(
    requestMessage: proto_api_v1_robot_pb.SensorReadingsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.SensorReadingsResponse|null) => void
  ): UnaryResponse;
  compassHeading(
    requestMessage: proto_api_v1_robot_pb.CompassHeadingRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassHeadingResponse|null) => void
  ): UnaryResponse;
  compassHeading(
    requestMessage: proto_api_v1_robot_pb.CompassHeadingRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassHeadingResponse|null) => void
  ): UnaryResponse;
  compassStartCalibration(
    requestMessage: proto_api_v1_robot_pb.CompassStartCalibrationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassStartCalibrationResponse|null) => void
  ): UnaryResponse;
  compassStartCalibration(
    requestMessage: proto_api_v1_robot_pb.CompassStartCalibrationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassStartCalibrationResponse|null) => void
  ): UnaryResponse;
  compassStopCalibration(
    requestMessage: proto_api_v1_robot_pb.CompassStopCalibrationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassStopCalibrationResponse|null) => void
  ): UnaryResponse;
  compassStopCalibration(
    requestMessage: proto_api_v1_robot_pb.CompassStopCalibrationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassStopCalibrationResponse|null) => void
  ): UnaryResponse;
  compassMark(
    requestMessage: proto_api_v1_robot_pb.CompassMarkRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassMarkResponse|null) => void
  ): UnaryResponse;
  compassMark(
    requestMessage: proto_api_v1_robot_pb.CompassMarkRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.CompassMarkResponse|null) => void
  ): UnaryResponse;
  forceMatrixMatrix(
    requestMessage: proto_api_v1_robot_pb.ForceMatrixMatrixRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ForceMatrixMatrixResponse|null) => void
  ): UnaryResponse;
  forceMatrixMatrix(
    requestMessage: proto_api_v1_robot_pb.ForceMatrixMatrixRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ForceMatrixMatrixResponse|null) => void
  ): UnaryResponse;
  forceMatrixSlipDetection(
    requestMessage: proto_api_v1_robot_pb.ForceMatrixSlipDetectionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ForceMatrixSlipDetectionResponse|null) => void
  ): UnaryResponse;
  forceMatrixSlipDetection(
    requestMessage: proto_api_v1_robot_pb.ForceMatrixSlipDetectionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ForceMatrixSlipDetectionResponse|null) => void
  ): UnaryResponse;
  executeFunction(
    requestMessage: proto_api_v1_robot_pb.ExecuteFunctionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ExecuteFunctionResponse|null) => void
  ): UnaryResponse;
  executeFunction(
    requestMessage: proto_api_v1_robot_pb.ExecuteFunctionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ExecuteFunctionResponse|null) => void
  ): UnaryResponse;
  executeSource(
    requestMessage: proto_api_v1_robot_pb.ExecuteSourceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ExecuteSourceResponse|null) => void
  ): UnaryResponse;
  executeSource(
    requestMessage: proto_api_v1_robot_pb.ExecuteSourceRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ExecuteSourceResponse|null) => void
  ): UnaryResponse;
  servoMove(
    requestMessage: proto_api_v1_robot_pb.ServoMoveRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ServoMoveResponse|null) => void
  ): UnaryResponse;
  servoMove(
    requestMessage: proto_api_v1_robot_pb.ServoMoveRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ServoMoveResponse|null) => void
  ): UnaryResponse;
  servoCurrent(
    requestMessage: proto_api_v1_robot_pb.ServoCurrentRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ServoCurrentResponse|null) => void
  ): UnaryResponse;
  servoCurrent(
    requestMessage: proto_api_v1_robot_pb.ServoCurrentRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ServoCurrentResponse|null) => void
  ): UnaryResponse;
  motorGetPIDConfig(
    requestMessage: proto_api_v1_robot_pb.MotorGetPIDConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGetPIDConfigResponse|null) => void
  ): UnaryResponse;
  motorGetPIDConfig(
    requestMessage: proto_api_v1_robot_pb.MotorGetPIDConfigRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGetPIDConfigResponse|null) => void
  ): UnaryResponse;
  motorSetPIDConfig(
    requestMessage: proto_api_v1_robot_pb.MotorSetPIDConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorSetPIDConfigResponse|null) => void
  ): UnaryResponse;
  motorSetPIDConfig(
    requestMessage: proto_api_v1_robot_pb.MotorSetPIDConfigRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorSetPIDConfigResponse|null) => void
  ): UnaryResponse;
  motorPIDStep(requestMessage: proto_api_v1_robot_pb.MotorPIDStepRequest, metadata?: grpc.Metadata): ResponseStream<proto_api_v1_robot_pb.MotorPIDStepResponse>;
  motorPower(
    requestMessage: proto_api_v1_robot_pb.MotorPowerRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorPowerResponse|null) => void
  ): UnaryResponse;
  motorPower(
    requestMessage: proto_api_v1_robot_pb.MotorPowerRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorPowerResponse|null) => void
  ): UnaryResponse;
  motorGo(
    requestMessage: proto_api_v1_robot_pb.MotorGoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoResponse|null) => void
  ): UnaryResponse;
  motorGo(
    requestMessage: proto_api_v1_robot_pb.MotorGoRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoResponse|null) => void
  ): UnaryResponse;
  motorGoFor(
    requestMessage: proto_api_v1_robot_pb.MotorGoForRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoForResponse|null) => void
  ): UnaryResponse;
  motorGoFor(
    requestMessage: proto_api_v1_robot_pb.MotorGoForRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoForResponse|null) => void
  ): UnaryResponse;
  motorGoTo(
    requestMessage: proto_api_v1_robot_pb.MotorGoToRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoToResponse|null) => void
  ): UnaryResponse;
  motorGoTo(
    requestMessage: proto_api_v1_robot_pb.MotorGoToRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoToResponse|null) => void
  ): UnaryResponse;
  motorGoTillStop(
    requestMessage: proto_api_v1_robot_pb.MotorGoTillStopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoTillStopResponse|null) => void
  ): UnaryResponse;
  motorGoTillStop(
    requestMessage: proto_api_v1_robot_pb.MotorGoTillStopRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorGoTillStopResponse|null) => void
  ): UnaryResponse;
  motorZero(
    requestMessage: proto_api_v1_robot_pb.MotorZeroRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorZeroResponse|null) => void
  ): UnaryResponse;
  motorZero(
    requestMessage: proto_api_v1_robot_pb.MotorZeroRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorZeroResponse|null) => void
  ): UnaryResponse;
  motorPosition(
    requestMessage: proto_api_v1_robot_pb.MotorPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorPositionResponse|null) => void
  ): UnaryResponse;
  motorPosition(
    requestMessage: proto_api_v1_robot_pb.MotorPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorPositionResponse|null) => void
  ): UnaryResponse;
  motorPositionSupported(
    requestMessage: proto_api_v1_robot_pb.MotorPositionSupportedRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorPositionSupportedResponse|null) => void
  ): UnaryResponse;
  motorPositionSupported(
    requestMessage: proto_api_v1_robot_pb.MotorPositionSupportedRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorPositionSupportedResponse|null) => void
  ): UnaryResponse;
  motorOff(
    requestMessage: proto_api_v1_robot_pb.MotorOffRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorOffResponse|null) => void
  ): UnaryResponse;
  motorOff(
    requestMessage: proto_api_v1_robot_pb.MotorOffRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorOffResponse|null) => void
  ): UnaryResponse;
  motorIsOn(
    requestMessage: proto_api_v1_robot_pb.MotorIsOnRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorIsOnResponse|null) => void
  ): UnaryResponse;
  motorIsOn(
    requestMessage: proto_api_v1_robot_pb.MotorIsOnRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.MotorIsOnResponse|null) => void
  ): UnaryResponse;
  inputControllerControls(
    requestMessage: proto_api_v1_robot_pb.InputControllerControlsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.InputControllerControlsResponse|null) => void
  ): UnaryResponse;
  inputControllerControls(
    requestMessage: proto_api_v1_robot_pb.InputControllerControlsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.InputControllerControlsResponse|null) => void
  ): UnaryResponse;
  inputControllerLastEvents(
    requestMessage: proto_api_v1_robot_pb.InputControllerLastEventsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.InputControllerLastEventsResponse|null) => void
  ): UnaryResponse;
  inputControllerLastEvents(
    requestMessage: proto_api_v1_robot_pb.InputControllerLastEventsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.InputControllerLastEventsResponse|null) => void
  ): UnaryResponse;
  inputControllerEventStream(requestMessage: proto_api_v1_robot_pb.InputControllerEventStreamRequest, metadata?: grpc.Metadata): ResponseStream<proto_api_v1_robot_pb.InputControllerEvent>;
  inputControllerInjectEvent(
    requestMessage: proto_api_v1_robot_pb.InputControllerInjectEventRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.InputControllerInjectEventResponse|null) => void
  ): UnaryResponse;
  inputControllerInjectEvent(
    requestMessage: proto_api_v1_robot_pb.InputControllerInjectEventRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.InputControllerInjectEventResponse|null) => void
  ): UnaryResponse;
  resourceRunCommand(
    requestMessage: proto_api_v1_robot_pb.ResourceRunCommandRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ResourceRunCommandResponse|null) => void
  ): UnaryResponse;
  resourceRunCommand(
    requestMessage: proto_api_v1_robot_pb.ResourceRunCommandRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ResourceRunCommandResponse|null) => void
  ): UnaryResponse;
  frameServiceConfig(
    requestMessage: proto_api_v1_robot_pb.FrameServiceConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.FrameServiceConfigResponse|null) => void
  ): UnaryResponse;
  frameServiceConfig(
    requestMessage: proto_api_v1_robot_pb.FrameServiceConfigRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.FrameServiceConfigResponse|null) => void
  ): UnaryResponse;
  navigationServiceMode(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceModeRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceModeResponse|null) => void
  ): UnaryResponse;
  navigationServiceMode(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceModeRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceModeResponse|null) => void
  ): UnaryResponse;
  navigationServiceSetMode(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceSetModeRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceSetModeResponse|null) => void
  ): UnaryResponse;
  navigationServiceSetMode(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceSetModeRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceSetModeResponse|null) => void
  ): UnaryResponse;
  navigationServiceLocation(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceLocationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceLocationResponse|null) => void
  ): UnaryResponse;
  navigationServiceLocation(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceLocationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceLocationResponse|null) => void
  ): UnaryResponse;
  navigationServiceWaypoints(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceWaypointsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceWaypointsResponse|null) => void
  ): UnaryResponse;
  navigationServiceWaypoints(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceWaypointsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceWaypointsResponse|null) => void
  ): UnaryResponse;
  navigationServiceAddWaypoint(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceAddWaypointRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceAddWaypointResponse|null) => void
  ): UnaryResponse;
  navigationServiceAddWaypoint(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceAddWaypointRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceAddWaypointResponse|null) => void
  ): UnaryResponse;
  navigationServiceRemoveWaypoint(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceRemoveWaypointRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceRemoveWaypointResponse|null) => void
  ): UnaryResponse;
  navigationServiceRemoveWaypoint(
    requestMessage: proto_api_v1_robot_pb.NavigationServiceRemoveWaypointRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.NavigationServiceRemoveWaypointResponse|null) => void
  ): UnaryResponse;
  objectManipulationServiceDoGrab(
    requestMessage: proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabResponse|null) => void
  ): UnaryResponse;
  objectManipulationServiceDoGrab(
    requestMessage: proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabResponse|null) => void
  ): UnaryResponse;
  iMUAngularVelocity(
    requestMessage: proto_api_v1_robot_pb.IMUAngularVelocityRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.IMUAngularVelocityResponse|null) => void
  ): UnaryResponse;
  iMUAngularVelocity(
    requestMessage: proto_api_v1_robot_pb.IMUAngularVelocityRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.IMUAngularVelocityResponse|null) => void
  ): UnaryResponse;
  iMUOrientation(
    requestMessage: proto_api_v1_robot_pb.IMUOrientationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.IMUOrientationResponse|null) => void
  ): UnaryResponse;
  iMUOrientation(
    requestMessage: proto_api_v1_robot_pb.IMUOrientationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.IMUOrientationResponse|null) => void
  ): UnaryResponse;
  gPSLocation(
    requestMessage: proto_api_v1_robot_pb.GPSLocationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSLocationResponse|null) => void
  ): UnaryResponse;
  gPSLocation(
    requestMessage: proto_api_v1_robot_pb.GPSLocationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSLocationResponse|null) => void
  ): UnaryResponse;
  gPSAltitude(
    requestMessage: proto_api_v1_robot_pb.GPSAltitudeRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSAltitudeResponse|null) => void
  ): UnaryResponse;
  gPSAltitude(
    requestMessage: proto_api_v1_robot_pb.GPSAltitudeRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSAltitudeResponse|null) => void
  ): UnaryResponse;
  gPSSpeed(
    requestMessage: proto_api_v1_robot_pb.GPSSpeedRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSSpeedResponse|null) => void
  ): UnaryResponse;
  gPSSpeed(
    requestMessage: proto_api_v1_robot_pb.GPSSpeedRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSSpeedResponse|null) => void
  ): UnaryResponse;
  gPSAccuracy(
    requestMessage: proto_api_v1_robot_pb.GPSAccuracyRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSAccuracyResponse|null) => void
  ): UnaryResponse;
  gPSAccuracy(
    requestMessage: proto_api_v1_robot_pb.GPSAccuracyRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.GPSAccuracyResponse|null) => void
  ): UnaryResponse;
}

