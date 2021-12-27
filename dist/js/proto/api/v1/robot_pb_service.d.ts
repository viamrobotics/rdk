// package: proto.api.v1
// file: proto/api/v1/robot.proto

import * as proto_api_v1_robot_pb from "../../../proto/api/v1/robot_pb";
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

