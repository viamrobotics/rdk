// package: proto.api.component.v1
// file: proto/api/component/v1/arm.proto

import * as proto_api_component_v1_arm_pb from "../../../../proto/api/component/v1/arm_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ArmServiceCurrentPosition = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceCurrentPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceCurrentPositionResponse;
};

type ArmServiceMoveToPosition = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToPositionResponse;
};

type ArmServiceCurrentJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsResponse;
};

type ArmServiceMoveToJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsResponse;
};

type ArmServiceJointMoveDelta = {
  readonly methodName: string;
  readonly service: typeof ArmService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaRequest;
  readonly responseType: typeof proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaResponse;
};

export class ArmService {
  static readonly serviceName: string;
  static readonly CurrentPosition: ArmServiceCurrentPosition;
  static readonly MoveToPosition: ArmServiceMoveToPosition;
  static readonly CurrentJointPositions: ArmServiceCurrentJointPositions;
  static readonly MoveToJointPositions: ArmServiceMoveToJointPositions;
  static readonly JointMoveDelta: ArmServiceJointMoveDelta;
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
  currentPosition(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentPositionResponse|null) => void
  ): UnaryResponse;
  currentPosition(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentPositionResponse|null) => void
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
  currentJointPositions(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsResponse|null) => void
  ): UnaryResponse;
  currentJointPositions(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsResponse|null) => void
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
  jointMoveDelta(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaResponse|null) => void
  ): UnaryResponse;
  jointMoveDelta(
    requestMessage: proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaResponse|null) => void
  ): UnaryResponse;
}

