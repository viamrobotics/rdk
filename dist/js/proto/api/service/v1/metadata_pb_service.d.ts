// package: proto.api.service.v1
// file: proto/api/service/v1/metadata.proto

import * as proto_api_service_v1_metadata_pb from "../../../../proto/api/service/v1/metadata_pb";
import {grpc} from "@improbable-eng/grpc-web";

type MetadataServiceResources = {
  readonly methodName: string;
  readonly service: typeof MetadataService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_service_v1_metadata_pb.ResourcesRequest;
  readonly responseType: typeof proto_api_service_v1_metadata_pb.ResourcesResponse;
};

export class MetadataService {
  static readonly serviceName: string;
  static readonly Resources: MetadataServiceResources;
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

export class MetadataServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  resources(
    requestMessage: proto_api_service_v1_metadata_pb.ResourcesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_service_v1_metadata_pb.ResourcesResponse|null) => void
  ): UnaryResponse;
  resources(
    requestMessage: proto_api_service_v1_metadata_pb.ResourcesRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_service_v1_metadata_pb.ResourcesResponse|null) => void
  ): UnaryResponse;
}

