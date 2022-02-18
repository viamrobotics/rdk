// package: proto.api.component.v1
// file: proto/api/component/v1/gantry.proto

import * as proto_api_component_v1_gantry_pb from "../../../../proto/api/component/v1/gantry_pb";
import {grpc} from "@improbable-eng/grpc-web";

type GantryServiceGetPosition = {
  readonly methodName: string;
  readonly service: typeof GantryService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gantry_pb.GantryServiceGetPositionRequest;
  readonly responseType: typeof proto_api_component_v1_gantry_pb.GantryServiceGetPositionResponse;
};

type GantryServiceMoveToPosition = {
  readonly methodName: string;
  readonly service: typeof GantryService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionRequest;
  readonly responseType: typeof proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionResponse;
};

type GantryServiceGetLengths = {
  readonly methodName: string;
  readonly service: typeof GantryService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gantry_pb.GantryServiceGetLengthsRequest;
  readonly responseType: typeof proto_api_component_v1_gantry_pb.GantryServiceGetLengthsResponse;
};

export class GantryService {
  static readonly serviceName: string;
  static readonly GetPosition: GantryServiceGetPosition;
  static readonly MoveToPosition: GantryServiceMoveToPosition;
  static readonly GetLengths: GantryServiceGetLengths;
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

export class GantryServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  getPosition(
    requestMessage: proto_api_component_v1_gantry_pb.GantryServiceGetPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gantry_pb.GantryServiceGetPositionResponse|null) => void
  ): UnaryResponse;
  getPosition(
    requestMessage: proto_api_component_v1_gantry_pb.GantryServiceGetPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gantry_pb.GantryServiceGetPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionResponse|null) => void
  ): UnaryResponse;
  getLengths(
    requestMessage: proto_api_component_v1_gantry_pb.GantryServiceGetLengthsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gantry_pb.GantryServiceGetLengthsResponse|null) => void
  ): UnaryResponse;
  getLengths(
    requestMessage: proto_api_component_v1_gantry_pb.GantryServiceGetLengthsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gantry_pb.GantryServiceGetLengthsResponse|null) => void
  ): UnaryResponse;
}

