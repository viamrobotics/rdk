// package: proto.api.component.v1
// file: proto/api/component/v1/arm.proto

import * as proto_api_component_v1_arm_pb from "../../../../proto/api/component/v1/arm_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ArmServiceGetEndPosition = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceGetEndPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceGetEndPositionResponse;
};

type ArmServiceMoveToPosition = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToPositionResponse;
};

type ArmServiceGetJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsResponse;
};

type ArmServiceMoveToJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsResponse;
};

export class ArmService {
  static readonly serviceName: string;
  static readonly GetEndPosition: ArmServiceGetEndPosition;
  static readonly MoveToPosition: ArmServiceMoveToPosition;
  static readonly GetJointPositions: ArmServiceGetJointPositions;
  static readonly MoveToJointPositions: ArmServiceMoveToJointPositions;
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

export class ArmServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  getEndPosition(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceGetEndPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceGetEndPositionResponse|null) => void
  ): UnaryResponse;
  getEndPosition(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceGetEndPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceGetEndPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionResponse|null) => void
  ): UnaryResponse;
  getJointPositions(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsResponse|null) => void
  ): UnaryResponse;
  getJointPositions(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsResponse|null) => void
  ): UnaryResponse;
  moveToJointPositions(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
  moveToJointPositions(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
}

