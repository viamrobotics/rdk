// package: proto.api.component.v1
// file: proto/api/component/v1/arm_subtype.proto

import * as proto_api_component_v1_arm_subtype_pb from "../../../../proto/api/component/v1/arm_subtype_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ArmSubtypeServiceCurrentPosition = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentPositionResponse;
};

type ArmSubtypeServiceMoveToPosition = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToPositionResponse;
};

type ArmSubtypeServiceCurrentJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentJointPositionsResponse;
};

type ArmSubtypeServiceMoveToJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToJointPositionsResponse;
};

type ArmSubtypeServiceJointMoveDelta = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceJointMoveDeltaRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceJointMoveDeltaResponse;
};

export class ArmSubtypeService {
  static readonly serviceName: string;
  static readonly CurrentPosition: ArmSubtypeServiceCurrentPosition;
  static readonly MoveToPosition: ArmSubtypeServiceMoveToPosition;
  static readonly CurrentJointPositions: ArmSubtypeServiceCurrentJointPositions;
  static readonly MoveToJointPositions: ArmSubtypeServiceMoveToJointPositions;
  static readonly JointMoveDelta: ArmSubtypeServiceJointMoveDelta;
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

export class ArmSubtypeServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  currentPosition(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentPositionResponse|null) => void
  ): UnaryResponse;
  currentPosition(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToPositionResponse|null) => void
  ): UnaryResponse;
  currentJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentJointPositionsResponse|null) => void
  ): UnaryResponse;
  currentJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceCurrentJointPositionsResponse|null) => void
  ): UnaryResponse;
  moveToJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
  moveToJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceMoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
  jointMoveDelta(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceJointMoveDeltaRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceJointMoveDeltaResponse|null) => void
  ): UnaryResponse;
  jointMoveDelta(
    requestMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceJointMoveDeltaRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.ArmSubtypeServiceJointMoveDeltaResponse|null) => void
  ): UnaryResponse;
}

