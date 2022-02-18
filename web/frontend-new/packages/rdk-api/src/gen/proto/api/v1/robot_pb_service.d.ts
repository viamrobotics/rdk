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

type RobotServiceSensorReadings = {
  readonly methodName: string;
  readonly service: typeof RobotService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_v1_robot_pb.SensorReadingsRequest;
  readonly responseType: typeof proto_api_v1_robot_pb.SensorReadingsResponse;
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

export class RobotService {
  static readonly serviceName: string;
  static readonly Status: RobotServiceStatus;
  static readonly StatusStream: RobotServiceStatusStream;
  static readonly Config: RobotServiceConfig;
  static readonly DoAction: RobotServiceDoAction;
  static readonly SensorReadings: RobotServiceSensorReadings;
  static readonly ExecuteFunction: RobotServiceExecuteFunction;
  static readonly ExecuteSource: RobotServiceExecuteSource;
  static readonly ResourceRunCommand: RobotServiceResourceRunCommand;
  static readonly FrameServiceConfig: RobotServiceFrameServiceConfig;
  static readonly NavigationServiceMode: RobotServiceNavigationServiceMode;
  static readonly NavigationServiceSetMode: RobotServiceNavigationServiceSetMode;
  static readonly NavigationServiceLocation: RobotServiceNavigationServiceLocation;
  static readonly NavigationServiceWaypoints: RobotServiceNavigationServiceWaypoints;
  static readonly NavigationServiceAddWaypoint: RobotServiceNavigationServiceAddWaypoint;
  static readonly NavigationServiceRemoveWaypoint: RobotServiceNavigationServiceRemoveWaypoint;
  static readonly ObjectManipulationServiceDoGrab: RobotServiceObjectManipulationServiceDoGrab;
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
  sensorReadings(
    requestMessage: proto_api_v1_robot_pb.SensorReadingsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.SensorReadingsResponse|null) => void
  ): UnaryResponse;
  sensorReadings(
    requestMessage: proto_api_v1_robot_pb.SensorReadingsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_v1_robot_pb.SensorReadingsResponse|null) => void
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
}

