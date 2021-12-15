// package: proto.api.component.v1
// file: proto/api/component/v1/gripper.proto

import * as proto_api_component_v1_gripper_pb from "../../../../proto/api/component/v1/gripper_pb";
import {grpc} from "@improbable-eng/grpc-web";

type GripperServiceOpen = {
  readonly methodName: string;
  readonly service: typeof GripperService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gripper_pb.GripperServiceOpenRequest;
  readonly responseType: typeof proto_api_component_v1_gripper_pb.GripperServiceOpenResponse;
};

type GripperServiceGrab = {
  readonly methodName: string;
  readonly service: typeof GripperService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gripper_pb.GripperServiceGrabRequest;
  readonly responseType: typeof proto_api_component_v1_gripper_pb.GripperServiceGrabResponse;
};

export class GripperService {
  static readonly serviceName: string;
  static readonly Open: GripperServiceOpen;
  static readonly Grab: GripperServiceGrab;
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

export class GripperServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  open(
    requestMessage: proto_api_component_v1_gripper_pb.GripperServiceOpenRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gripper_pb.GripperServiceOpenResponse|null) => void
  ): UnaryResponse;
  open(
    requestMessage: proto_api_component_v1_gripper_pb.GripperServiceOpenRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gripper_pb.GripperServiceOpenResponse|null) => void
  ): UnaryResponse;
  grab(
    requestMessage: proto_api_component_v1_gripper_pb.GripperServiceGrabRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gripper_pb.GripperServiceGrabResponse|null) => void
  ): UnaryResponse;
  grab(
    requestMessage: proto_api_component_v1_gripper_pb.GripperServiceGrabRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gripper_pb.GripperServiceGrabResponse|null) => void
  ): UnaryResponse;
}

