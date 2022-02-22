// package: proto.api.component.v1
// file: proto/api/component/v1/motor.proto

import * as proto_api_component_v1_motor_pb from "../../../../proto/api/component/v1/motor_pb";
import {grpc} from "@improbable-eng/grpc-web";

type MotorServiceSetPower = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceSetPowerRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceSetPowerResponse;
};

type MotorServiceGo = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceGoRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceGoResponse;
};

type MotorServiceGoFor = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceGoForRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceGoForResponse;
};

type MotorServiceGoTo = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceGoToRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceGoToResponse;
};

type MotorServiceGoTillStop = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceGoTillStopRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceGoTillStopResponse;
};

type MotorServiceResetZeroPosition = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionResponse;
};

type MotorServicePosition = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServicePositionRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServicePositionResponse;
};

type MotorServicePositionSupported = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServicePositionSupportedRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServicePositionSupportedResponse;
};

type MotorServiceStop = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceStopRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceStopResponse;
};

type MotorServiceIsOn = {
  readonly methodName: string;
  readonly service: typeof MotorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_motor_pb.MotorServiceIsOnRequest;
  readonly responseType: typeof proto_api_component_v1_motor_pb.MotorServiceIsOnResponse;
};

export class MotorService {
  static readonly serviceName: string;
  static readonly SetPower: MotorServiceSetPower;
  static readonly Go: MotorServiceGo;
  static readonly GoFor: MotorServiceGoFor;
  static readonly GoTo: MotorServiceGoTo;
  static readonly GoTillStop: MotorServiceGoTillStop;
  static readonly ResetZeroPosition: MotorServiceResetZeroPosition;
  static readonly Position: MotorServicePosition;
  static readonly PositionSupported: MotorServicePositionSupported;
  static readonly Stop: MotorServiceStop;
  static readonly IsOn: MotorServiceIsOn;
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

export class MotorServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  setPower(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceSetPowerRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceSetPowerResponse|null) => void
  ): UnaryResponse;
  setPower(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceSetPowerRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceSetPowerResponse|null) => void
  ): UnaryResponse;
  go(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoResponse|null) => void
  ): UnaryResponse;
  go(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoResponse|null) => void
  ): UnaryResponse;
  goFor(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoForRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoForResponse|null) => void
  ): UnaryResponse;
  goFor(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoForRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoForResponse|null) => void
  ): UnaryResponse;
  goTo(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoToRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoToResponse|null) => void
  ): UnaryResponse;
  goTo(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoToRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoToResponse|null) => void
  ): UnaryResponse;
  goTillStop(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoTillStopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoTillStopResponse|null) => void
  ): UnaryResponse;
  goTillStop(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceGoTillStopRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceGoTillStopResponse|null) => void
  ): UnaryResponse;
  resetZeroPosition(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionResponse|null) => void
  ): UnaryResponse;
  resetZeroPosition(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionResponse|null) => void
  ): UnaryResponse;
  position(
    requestMessage: proto_api_component_v1_motor_pb.MotorServicePositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServicePositionResponse|null) => void
  ): UnaryResponse;
  position(
    requestMessage: proto_api_component_v1_motor_pb.MotorServicePositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServicePositionResponse|null) => void
  ): UnaryResponse;
  positionSupported(
    requestMessage: proto_api_component_v1_motor_pb.MotorServicePositionSupportedRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServicePositionSupportedResponse|null) => void
  ): UnaryResponse;
  positionSupported(
    requestMessage: proto_api_component_v1_motor_pb.MotorServicePositionSupportedRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServicePositionSupportedResponse|null) => void
  ): UnaryResponse;
  stop(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceStopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceStopResponse|null) => void
  ): UnaryResponse;
  stop(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceStopRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceStopResponse|null) => void
  ): UnaryResponse;
  isOn(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceIsOnRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceIsOnResponse|null) => void
  ): UnaryResponse;
  isOn(
    requestMessage: proto_api_component_v1_motor_pb.MotorServiceIsOnRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_motor_pb.MotorServiceIsOnResponse|null) => void
  ): UnaryResponse;
}

