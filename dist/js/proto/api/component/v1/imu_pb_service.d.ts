// package: proto.api.component.v1
// file: proto/api/component/v1/imu.proto

import * as proto_api_component_v1_imu_pb from "../../../../proto/api/component/v1/imu_pb";
import {grpc} from "@improbable-eng/grpc-web";

type IMUServiceIMUAngularVelocity = {
  readonly methodName: string;
  readonly service: typeof IMUService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_imu_pb.IMUAngularVelocityRequest;
  readonly responseType: typeof proto_api_component_v1_imu_pb.IMUAngularVelocityResponse;
};

type IMUServiceIMUOrientation = {
  readonly methodName: string;
  readonly service: typeof IMUService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_imu_pb.IMUOrientationRequest;
  readonly responseType: typeof proto_api_component_v1_imu_pb.IMUOrientationResponse;
};

export class IMUService {
  static readonly serviceName: string;
  static readonly IMUAngularVelocity: IMUServiceIMUAngularVelocity;
  static readonly IMUOrientation: IMUServiceIMUOrientation;
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
  iMUAngularVelocity(
    requestMessage: proto_api_component_v1_imu_pb.IMUAngularVelocityRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUAngularVelocityResponse|null) => void
  ): UnaryResponse;
  iMUAngularVelocity(
    requestMessage: proto_api_component_v1_imu_pb.IMUAngularVelocityRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUAngularVelocityResponse|null) => void
  ): UnaryResponse;
  iMUOrientation(
    requestMessage: proto_api_component_v1_imu_pb.IMUOrientationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUOrientationResponse|null) => void
  ): UnaryResponse;
  iMUOrientation(
    requestMessage: proto_api_component_v1_imu_pb.IMUOrientationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_imu_pb.IMUOrientationResponse|null) => void
  ): UnaryResponse;
}

