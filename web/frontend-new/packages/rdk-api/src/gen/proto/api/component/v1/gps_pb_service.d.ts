// package: proto.api.component.v1
// file: proto/api/component/v1/gps.proto

import * as proto_api_component_v1_gps_pb from "../../../../proto/api/component/v1/gps_pb";
import {grpc} from "@improbable-eng/grpc-web";

type GPSServiceReadLocation = {
  readonly methodName: string;
  readonly service: typeof GPSService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gps_pb.GPSServiceReadLocationRequest;
  readonly responseType: typeof proto_api_component_v1_gps_pb.GPSServiceReadLocationResponse;
};

type GPSServiceReadAltitude = {
  readonly methodName: string;
  readonly service: typeof GPSService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gps_pb.GPSServiceReadAltitudeRequest;
  readonly responseType: typeof proto_api_component_v1_gps_pb.GPSServiceReadAltitudeResponse;
};

type GPSServiceReadSpeed = {
  readonly methodName: string;
  readonly service: typeof GPSService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_gps_pb.GPSServiceReadSpeedRequest;
  readonly responseType: typeof proto_api_component_v1_gps_pb.GPSServiceReadSpeedResponse;
};

export class GPSService {
  static readonly serviceName: string;
  static readonly ReadLocation: GPSServiceReadLocation;
  static readonly ReadAltitude: GPSServiceReadAltitude;
  static readonly ReadSpeed: GPSServiceReadSpeed;
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

export class GPSServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  readLocation(
    requestMessage: proto_api_component_v1_gps_pb.GPSServiceReadLocationRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gps_pb.GPSServiceReadLocationResponse|null) => void
  ): UnaryResponse;
  readLocation(
    requestMessage: proto_api_component_v1_gps_pb.GPSServiceReadLocationRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gps_pb.GPSServiceReadLocationResponse|null) => void
  ): UnaryResponse;
  readAltitude(
    requestMessage: proto_api_component_v1_gps_pb.GPSServiceReadAltitudeRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gps_pb.GPSServiceReadAltitudeResponse|null) => void
  ): UnaryResponse;
  readAltitude(
    requestMessage: proto_api_component_v1_gps_pb.GPSServiceReadAltitudeRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gps_pb.GPSServiceReadAltitudeResponse|null) => void
  ): UnaryResponse;
  readSpeed(
    requestMessage: proto_api_component_v1_gps_pb.GPSServiceReadSpeedRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gps_pb.GPSServiceReadSpeedResponse|null) => void
  ): UnaryResponse;
  readSpeed(
    requestMessage: proto_api_component_v1_gps_pb.GPSServiceReadSpeedRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_gps_pb.GPSServiceReadSpeedResponse|null) => void
  ): UnaryResponse;
}

