// package: proto.api.component.v1
// file: proto/api/component/v1/sensor.proto

import * as proto_api_component_v1_sensor_pb from "../../../../proto/api/component/v1/sensor_pb";
import {grpc} from "@improbable-eng/grpc-web";

type SensorServiceGetReadings = {
  readonly methodName: string;
  readonly service: typeof SensorService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_sensor_pb.SensorServiceGetReadingsRequest;
  readonly responseType: typeof proto_api_component_v1_sensor_pb.SensorServiceGetReadingsResponse;
};

export class SensorService {
  static readonly serviceName: string;
  static readonly GetReadings: SensorServiceGetReadings;
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

export class SensorServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  getReadings(
    requestMessage: proto_api_component_v1_sensor_pb.SensorServiceGetReadingsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_sensor_pb.SensorServiceGetReadingsResponse|null) => void
  ): UnaryResponse;
  getReadings(
    requestMessage: proto_api_component_v1_sensor_pb.SensorServiceGetReadingsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_sensor_pb.SensorServiceGetReadingsResponse|null) => void
  ): UnaryResponse;
}

