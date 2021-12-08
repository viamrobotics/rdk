// package: proto.api.component.v1
// file: proto/api/component/v1/camera.proto

import * as proto_api_component_v1_camera_pb from "../../../../proto/api/component/v1/camera_pb";
import * as google_api_httpbody_pb from "../../../../google/api/httpbody_pb";
import {grpc} from "@improbable-eng/grpc-web";

type CameraServiceFrame = {
  readonly methodName: string;
  readonly service: typeof CameraService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_camera_pb.CameraServiceFrameRequest;
  readonly responseType: typeof proto_api_component_v1_camera_pb.CameraServiceFrameResponse;
};

type CameraServiceRenderFrame = {
  readonly methodName: string;
  readonly service: typeof CameraService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_camera_pb.CameraServiceRenderFrameRequest;
  readonly responseType: typeof google_api_httpbody_pb.HttpBody;
};

type CameraServicePointCloud = {
  readonly methodName: string;
  readonly service: typeof CameraService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_camera_pb.CameraServicePointCloudRequest;
  readonly responseType: typeof proto_api_component_v1_camera_pb.CameraServicePointCloudResponse;
};

type CameraServiceObjectPointClouds = {
  readonly methodName: string;
  readonly service: typeof CameraService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsRequest;
  readonly responseType: typeof proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsResponse;
};

export class CameraService {
  static readonly serviceName: string;
  static readonly Frame: CameraServiceFrame;
  static readonly RenderFrame: CameraServiceRenderFrame;
  static readonly PointCloud: CameraServicePointCloud;
  static readonly ObjectPointClouds: CameraServiceObjectPointClouds;
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

export class CameraServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  frame(
    requestMessage: proto_api_component_v1_camera_pb.CameraServiceFrameRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_camera_pb.CameraServiceFrameResponse|null) => void
  ): UnaryResponse;
  frame(
    requestMessage: proto_api_component_v1_camera_pb.CameraServiceFrameRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_camera_pb.CameraServiceFrameResponse|null) => void
  ): UnaryResponse;
  renderFrame(
    requestMessage: proto_api_component_v1_camera_pb.CameraServiceRenderFrameRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_httpbody_pb.HttpBody|null) => void
  ): UnaryResponse;
  renderFrame(
    requestMessage: proto_api_component_v1_camera_pb.CameraServiceRenderFrameRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_httpbody_pb.HttpBody|null) => void
  ): UnaryResponse;
  pointCloud(
    requestMessage: proto_api_component_v1_camera_pb.CameraServicePointCloudRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_camera_pb.CameraServicePointCloudResponse|null) => void
  ): UnaryResponse;
  pointCloud(
    requestMessage: proto_api_component_v1_camera_pb.CameraServicePointCloudRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_camera_pb.CameraServicePointCloudResponse|null) => void
  ): UnaryResponse;
  objectPointClouds(
    requestMessage: proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsResponse|null) => void
  ): UnaryResponse;
  objectPointClouds(
    requestMessage: proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsResponse|null) => void
  ): UnaryResponse;
}

