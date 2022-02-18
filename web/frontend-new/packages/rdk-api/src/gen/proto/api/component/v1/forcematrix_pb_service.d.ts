// package: proto.api.component.v1
// file: proto/api/component/v1/forcematrix.proto

import * as proto_api_component_v1_forcematrix_pb from "../../../../proto/api/component/v1/forcematrix_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ForceMatrixServiceReadMatrix = {
  readonly methodName: string;
  readonly service: typeof ForceMatrixService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixRequest;
  readonly responseType: typeof proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixResponse;
};

type ForceMatrixServiceDetectSlip = {
  readonly methodName: string;
  readonly service: typeof ForceMatrixService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipRequest;
  readonly responseType: typeof proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipResponse;
};

export class ForceMatrixService {
  static readonly serviceName: string;
  static readonly ReadMatrix: ForceMatrixServiceReadMatrix;
  static readonly DetectSlip: ForceMatrixServiceDetectSlip;
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

export class ForceMatrixServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  readMatrix(
    requestMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixResponse|null) => void
  ): UnaryResponse;
  readMatrix(
    requestMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixResponse|null) => void
  ): UnaryResponse;
  detectSlip(
    requestMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipResponse|null) => void
  ): UnaryResponse;
  detectSlip(
    requestMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipResponse|null) => void
  ): UnaryResponse;
}

