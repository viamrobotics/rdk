// package: proto.api.component.v1
// file: proto/api/component/v1/imu.proto

import * as proto_api_component_v1_imu_pb from "../../../../proto/api/component/v1/imu_pb";
import {grpc} from "@improbable-eng/grpc-web";

type IMUServiceAngularVelocity = {
  readonly methodName: string;
  readonly service: typeof IMUService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_imu_pb.IMUServiceAngularVelocityRequest;
  readonly responseType: typeof proto_api_component_v1_imu_pb.IMUServiceAngularVelocityResponse;
};

type IMUServiceOrientation = {
  readonly methodName: string;
  readonly service: typeof IMUService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_imu_pb.IMUServiceOrientationRequest;
  readonly responseType: typeof proto_api_component_v1_imu_pb.IMUServiceOrientationResponse;
};

export class IMUService {
  static readonly serviceName: string;
  static readonly AngularVelocity: IMUServiceAngularVelocity;
  static readonly Orientation: IMUServiceOrientation;
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

export class IMUServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  angularVelocity(
    requestMessage: proto_api_component_v1_imu_pb.IMUServiceAngularVelocityRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUServiceAngularVelocityResponse|null) => void
  ): UnaryResponse;
  angularVelocity(
    requestMessage: proto_api_component_v1_imu_pb.IMUServiceAngularVelocityRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUServiceAngularVelocityResponse|null) => void
  ): UnaryResponse;
  orientation(
    requestMessage: proto_api_component_v1_imu_pb.IMUServiceOrientationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUServiceOrientationResponse|null) => void
  ): UnaryResponse;
  orientation(
    requestMessage: proto_api_component_v1_imu_pb.IMUServiceOrientationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUServiceOrientationResponse|null) => void
  ): UnaryResponse;
}

