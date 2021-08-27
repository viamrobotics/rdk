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

type RobotServiceArmCurrentPosition = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ArmCurrentPositionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ArmCurrentPositionResponse;
};

type RobotServiceArmMoveToPosition = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ArmMoveToPositionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ArmMoveToPositionResponse;
};

type RobotServiceArmCurrentJointPositions = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ArmCurrentJointPositionsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ArmCurrentJointPositionsResponse;
};

type RobotServiceArmMoveToJointPositions = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ArmMoveToJointPositionsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ArmMoveToJointPositionsResponse;
};

type RobotServiceArmJointMoveDelta = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.ArmJointMoveDeltaRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.ArmJointMoveDeltaResponse;
};

type RobotServiceBaseMoveStraight = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BaseMoveStraightRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BaseMoveStraightResponse;
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

type RobotServiceBoardMotorPower = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorPowerRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorPowerResponse;
};

type RobotServiceBoardMotorGo = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorGoRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorGoResponse;
};

type RobotServiceBoardMotorGoFor = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorGoForRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorGoForResponse;
};

type RobotServiceBoardMotorGoTo = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorGoToRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorGoToResponse;
};

type RobotServiceBoardMotorGoTillStop = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorGoTillStopRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorGoTillStopResponse;
};

type RobotServiceBoardMotorZero = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorZeroRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorZeroResponse;
};

type RobotServiceBoardMotorPosition = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorPositionRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorPositionResponse;
};

type RobotServiceBoardMotorPositionSupported = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorPositionSupportedRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorPositionSupportedResponse;
};

type RobotServiceBoardMotorOff = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorOffRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorOffResponse;
};

type RobotServiceBoardMotorIsOn = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardMotorIsOnRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardMotorIsOnResponse;
};

type RobotServiceBoardServoMove = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardServoMoveRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardServoMoveResponse;
};

type RobotServiceBoardServoCurrent = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.BoardServoCurrentRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.BoardServoCurrentResponse;
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

export class RobotService {
  static readonly serviceName: string;
  static readonly Status: RobotServiceStatus;
  static readonly StatusStream: RobotServiceStatusStream;
  static readonly Config: RobotServiceConfig;
  static readonly DoAction: RobotServiceDoAction;
  static readonly ArmCurrentPosition: RobotServiceArmCurrentPosition;
  static readonly ArmMoveToPosition: RobotServiceArmMoveToPosition;
  static readonly ArmCurrentJointPositions: RobotServiceArmCurrentJointPositions;
  static readonly ArmMoveToJointPositions: RobotServiceArmMoveToJointPositions;
  static readonly ArmJointMoveDelta: RobotServiceArmJointMoveDelta;
  static readonly BaseMoveStraight: RobotServiceBaseMoveStraight;
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
  static readonly BoardMotorPower: RobotServiceBoardMotorPower;
  static readonly BoardMotorGo: RobotServiceBoardMotorGo;
  static readonly BoardMotorGoFor: RobotServiceBoardMotorGoFor;
  static readonly BoardMotorGoTo: RobotServiceBoardMotorGoTo;
  static readonly BoardMotorGoTillStop: RobotServiceBoardMotorGoTillStop;
  static readonly BoardMotorZero: RobotServiceBoardMotorZero;
  static readonly BoardMotorPosition: RobotServiceBoardMotorPosition;
  static readonly BoardMotorPositionSupported: RobotServiceBoardMotorPositionSupported;
  static readonly BoardMotorOff: RobotServiceBoardMotorOff;
  static readonly BoardMotorIsOn: RobotServiceBoardMotorIsOn;
  static readonly BoardServoMove: RobotServiceBoardServoMove;
  static readonly BoardServoCurrent: RobotServiceBoardServoCurrent;
  static readonly BoardAnalogReaderRead: RobotServiceBoardAnalogReaderRead;
  static readonly BoardDigitalInterruptConfig: RobotServiceBoardDigitalInterruptConfig;
  static readonly BoardDigitalInterruptValue: RobotServiceBoardDigitalInterruptValue;
  static readonly BoardDigitalInterruptTick: RobotServiceBoardDigitalInterruptTick;
  static readonly SensorReadings: RobotServiceSensorReadings;
  static readonly CompassHeading: RobotServiceCompassHeading;
  static readonly CompassStartCalibration: RobotServiceCompassStartCalibration;
  static readonly CompassStopCalibration: RobotServiceCompassStopCalibration;
  static readonly CompassMark: RobotServiceCompassMark;
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
  armCurrentPosition(
    requestMessage: proto_api_v1_robot_pb.ArmCurrentPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmCurrentPositionResponse|null) => void
  ): UnaryResponse;
  armCurrentPosition(
    requestMessage: proto_api_v1_robot_pb.ArmCurrentPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmCurrentPositionResponse|null) => void
  ): UnaryResponse;
  armMoveToPosition(
    requestMessage: proto_api_v1_robot_pb.ArmMoveToPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmMoveToPositionResponse|null) => void
  ): UnaryResponse;
  armMoveToPosition(
    requestMessage: proto_api_v1_robot_pb.ArmMoveToPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmMoveToPositionResponse|null) => void
  ): UnaryResponse;
  armCurrentJointPositions(
    requestMessage: proto_api_v1_robot_pb.ArmCurrentJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmCurrentJointPositionsResponse|null) => void
  ): UnaryResponse;
  armCurrentJointPositions(
    requestMessage: proto_api_v1_robot_pb.ArmCurrentJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmCurrentJointPositionsResponse|null) => void
  ): UnaryResponse;
  armMoveToJointPositions(
    requestMessage: proto_api_v1_robot_pb.ArmMoveToJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmMoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
  armMoveToJointPositions(
    requestMessage: proto_api_v1_robot_pb.ArmMoveToJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmMoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
  armJointMoveDelta(
    requestMessage: proto_api_v1_robot_pb.ArmJointMoveDeltaRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmJointMoveDeltaResponse|null) => void
  ): UnaryResponse;
  armJointMoveDelta(
    requestMessage: proto_api_v1_robot_pb.ArmJointMoveDeltaRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.ArmJointMoveDeltaResponse|null) => void
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
  boardMotorPower(
    requestMessage: proto_api_v1_robot_pb.BoardMotorPowerRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorPowerResponse|null) => void
  ): UnaryResponse;
  boardMotorPower(
    requestMessage: proto_api_v1_robot_pb.BoardMotorPowerRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorPowerResponse|null) => void
  ): UnaryResponse;
  boardMotorGo(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoResponse|null) => void
  ): UnaryResponse;
  boardMotorGo(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoResponse|null) => void
  ): UnaryResponse;
  boardMotorGoFor(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoForRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoForResponse|null) => void
  ): UnaryResponse;
  boardMotorGoFor(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoForRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoForResponse|null) => void
  ): UnaryResponse;
  boardMotorGoTo(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoToRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoToResponse|null) => void
  ): UnaryResponse;
  boardMotorGoTo(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoToRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoToResponse|null) => void
  ): UnaryResponse;
  boardMotorGoTillStop(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoTillStopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoTillStopResponse|null) => void
  ): UnaryResponse;
  boardMotorGoTillStop(
    requestMessage: proto_api_v1_robot_pb.BoardMotorGoTillStopRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorGoTillStopResponse|null) => void
  ): UnaryResponse;
  boardMotorZero(
    requestMessage: proto_api_v1_robot_pb.BoardMotorZeroRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorZeroResponse|null) => void
  ): UnaryResponse;
  boardMotorZero(
    requestMessage: proto_api_v1_robot_pb.BoardMotorZeroRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorZeroResponse|null) => void
  ): UnaryResponse;
  boardMotorPosition(
    requestMessage: proto_api_v1_robot_pb.BoardMotorPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorPositionResponse|null) => void
  ): UnaryResponse;
  boardMotorPosition(
    requestMessage: proto_api_v1_robot_pb.BoardMotorPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorPositionResponse|null) => void
  ): UnaryResponse;
  boardMotorPositionSupported(
    requestMessage: proto_api_v1_robot_pb.BoardMotorPositionSupportedRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorPositionSupportedResponse|null) => void
  ): UnaryResponse;
  boardMotorPositionSupported(
    requestMessage: proto_api_v1_robot_pb.BoardMotorPositionSupportedRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorPositionSupportedResponse|null) => void
  ): UnaryResponse;
  boardMotorOff(
    requestMessage: proto_api_v1_robot_pb.BoardMotorOffRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorOffResponse|null) => void
  ): UnaryResponse;
  boardMotorOff(
    requestMessage: proto_api_v1_robot_pb.BoardMotorOffRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorOffResponse|null) => void
  ): UnaryResponse;
  boardMotorIsOn(
    requestMessage: proto_api_v1_robot_pb.BoardMotorIsOnRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorIsOnResponse|null) => void
  ): UnaryResponse;
  boardMotorIsOn(
    requestMessage: proto_api_v1_robot_pb.BoardMotorIsOnRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardMotorIsOnResponse|null) => void
  ): UnaryResponse;
  boardServoMove(
    requestMessage: proto_api_v1_robot_pb.BoardServoMoveRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardServoMoveResponse|null) => void
  ): UnaryResponse;
  boardServoMove(
    requestMessage: proto_api_v1_robot_pb.BoardServoMoveRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardServoMoveResponse|null) => void
  ): UnaryResponse;
  boardServoCurrent(
    requestMessage: proto_api_v1_robot_pb.BoardServoCurrentRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardServoCurrentResponse|null) => void
  ): UnaryResponse;
  boardServoCurrent(
    requestMessage: proto_api_v1_robot_pb.BoardServoCurrentRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.BoardServoCurrentResponse|null) => void
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
}

