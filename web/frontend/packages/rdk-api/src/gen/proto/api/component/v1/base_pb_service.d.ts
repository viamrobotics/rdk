// package: proto.api.component.v1
// file: proto/api/component/v1/base.proto

import * as proto_api_component_v1_base_pb from "../../../../proto/api/component/v1/base_pb";
import {grpc} from "@improbable-eng/grpc-web";

type BaseServiceMoveStraight = {
  readonly methodName: string;
  readonly service: typeof BaseService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_base_pb.BaseServiceMoveStraightRequest;
  readonly responseType: typeof proto_api_component_v1_base_pb.BaseServiceMoveStraightResponse;
};

type BaseServiceMoveArc = {
  readonly methodName: string;
  readonly service: typeof BaseService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_base_pb.BaseServiceMoveArcRequest;
  readonly responseType: typeof proto_api_component_v1_base_pb.BaseServiceMoveArcResponse;
};

type BaseServiceSpin = {
  readonly methodName: string;
  readonly service: typeof BaseService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_base_pb.BaseServiceSpinRequest;
  readonly responseType: typeof proto_api_component_v1_base_pb.BaseServiceSpinResponse;
};

type BaseServiceStop = {
  readonly methodName: string;
  readonly service: typeof BaseService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_base_pb.BaseServiceStopRequest;
  readonly responseType: typeof proto_api_component_v1_base_pb.BaseServiceStopResponse;
};

export class BaseService {
  static readonly serviceName: string;
  static readonly MoveStraight: BaseServiceMoveStraight;
  static readonly MoveArc: BaseServiceMoveArc;
  static readonly Spin: BaseServiceSpin;
  static readonly Stop: BaseServiceStop;
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

export class BaseServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  moveStraight(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceMoveStraightRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceMoveStraightResponse|null) => void
  ): UnaryResponse;
  moveStraight(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceMoveStraightRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceMoveStraightResponse|null) => void
  ): UnaryResponse;
  moveArc(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceMoveArcRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceMoveArcResponse|null) => void
  ): UnaryResponse;
  moveArc(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceMoveArcRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceMoveArcResponse|null) => void
  ): UnaryResponse;
  spin(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceSpinRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceSpinResponse|null) => void
  ): UnaryResponse;
  spin(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceSpinRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceSpinResponse|null) => void
  ): UnaryResponse;
  stop(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceStopRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceStopResponse|null) => void
  ): UnaryResponse;
  stop(
    requestMessage: proto_api_component_v1_base_pb.BaseServiceStopRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_base_pb.BaseServiceStopResponse|null) => void
  ): UnaryResponse;
}

