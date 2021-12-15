// package: proto.api.component.v1
// file: proto/api/component/v1/servo.proto

import * as proto_api_component_v1_servo_pb from "../../../../proto/api/component/v1/servo_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ServoServiceMove = {
  readonly methodName: string;
  readonly service: typeof ServoService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_servo_pb.ServoServiceMoveRequest;
  readonly responseType: typeof proto_api_component_v1_servo_pb.ServoServiceMoveResponse;
};

type ServoServiceAngularOffset = {
  readonly methodName: string;
  readonly service: typeof ServoService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_servo_pb.ServoServiceAngularOffsetRequest;
  readonly responseType: typeof proto_api_component_v1_servo_pb.ServoServiceAngularOffsetResponse;
};

export class ServoService {
  static readonly serviceName: string;
  static readonly Move: ServoServiceMove;
  static readonly AngularOffset: ServoServiceAngularOffset;
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

export class ServoServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  move(
    requestMessage: proto_api_component_v1_servo_pb.ServoServiceMoveRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_servo_pb.ServoServiceMoveResponse|null) => void
  ): UnaryResponse;
  move(
    requestMessage: proto_api_component_v1_servo_pb.ServoServiceMoveRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_servo_pb.ServoServiceMoveResponse|null) => void
  ): UnaryResponse;
  angularOffset(
    requestMessage: proto_api_component_v1_servo_pb.ServoServiceAngularOffsetRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_servo_pb.ServoServiceAngularOffsetResponse|null) => void
  ): UnaryResponse;
  angularOffset(
    requestMessage: proto_api_component_v1_servo_pb.ServoServiceAngularOffsetRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_servo_pb.ServoServiceAngularOffsetResponse|null) => void
  ): UnaryResponse;
}

