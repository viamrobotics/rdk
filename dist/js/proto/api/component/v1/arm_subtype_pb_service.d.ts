// package: proto.api.component.v1
// file: proto/api/component/v1/arm_subtype.proto

import * as proto_api_component_v1_arm_subtype_pb from "../../../../proto/api/component/v1/arm_subtype_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ArmSubtypeServiceCurrentPosition = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.CurrentPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.CurrentPositionResponse;
};

type ArmSubtypeServiceMoveToPosition = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.MoveToPositionRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.MoveToPositionResponse;
};

type ArmSubtypeServiceCurrentJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsResponse;
};

type ArmSubtypeServiceMoveToJointPositions = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsResponse;
};

type ArmSubtypeServiceJointMoveDelta = {
  readonly methodName: string;
  readonly service: typeof ArmSubtypeService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_arm_subtype_pb.JointMoveDeltaRequest;
  readonly responseType: typeof proto_api_component_v1_arm_subtype_pb.JointMoveDeltaResponse;
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
    requestMessage: proto_api_component_v1_arm_subtype_pb.CurrentPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.CurrentPositionResponse|null) => void
  ): UnaryResponse;
  currentPosition(
    requestMessage: proto_api_component_v1_arm_subtype_pb.CurrentPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.CurrentPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_arm_subtype_pb.MoveToPositionRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.MoveToPositionResponse|null) => void
  ): UnaryResponse;
  moveToPosition(
    requestMessage: proto_api_component_v1_arm_subtype_pb.MoveToPositionRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.MoveToPositionResponse|null) => void
  ): UnaryResponse;
  currentJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsResponse|null) => void
  ): UnaryResponse;
  currentJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsResponse|null) => void
  ): UnaryResponse;
  moveToJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
  moveToJointPositions(
    requestMessage: proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsResponse|null) => void
  ): UnaryResponse;
  jointMoveDelta(
    requestMessage: proto_api_component_v1_arm_subtype_pb.JointMoveDeltaRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.JointMoveDeltaResponse|null) => void
  ): UnaryResponse;
  jointMoveDelta(
    requestMessage: proto_api_component_v1_arm_subtype_pb.JointMoveDeltaRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_arm_subtype_pb.JointMoveDeltaResponse|null) => void
  ): UnaryResponse;
}

