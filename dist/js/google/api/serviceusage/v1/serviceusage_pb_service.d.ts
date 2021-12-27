// package: google.api.serviceusage.v1
// file: google/api/serviceusage/v1/serviceusage.proto

import * as google_api_serviceusage_v1_serviceusage_pb from "../../../../google/api/serviceusage/v1/serviceusage_pb";
import * as google_api_serviceusage_v1_resources_pb from "../../../../google/api/serviceusage/v1/resources_pb";
import * as google_longrunning_operations_pb from "../../../../google/longrunning/operations_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ServiceUsageEnableService = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1_serviceusage_pb.EnableServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageDisableService = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1_serviceusage_pb.DisableServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageGetService = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1_serviceusage_pb.GetServiceRequest;
  readonly responseType: typeof google_api_serviceusage_v1_resources_pb.Service;
};

type ServiceUsageListServices = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1_serviceusage_pb.ListServicesRequest;
  readonly responseType: typeof google_api_serviceusage_v1_serviceusage_pb.ListServicesResponse;
};

type ServiceUsageBatchEnableServices = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1_serviceusage_pb.BatchEnableServicesRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageBatchGetServices = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesRequest;
  readonly responseType: typeof google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesResponse;
};

export class ServiceUsage {
  static readonly serviceName: string;
  static readonly EnableService: ServiceUsageEnableService;
  static readonly DisableService: ServiceUsageDisableService;
  static readonly GetService: ServiceUsageGetService;
  static readonly ListServices: ServiceUsageListServices;
  static readonly BatchEnableServices: ServiceUsageBatchEnableServices;
  static readonly BatchGetServices: ServiceUsageBatchGetServices;
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

export class ServiceUsageClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  enableService(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.EnableServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  enableService(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.EnableServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  disableService(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.DisableServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  disableService(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.DisableServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  getService(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.GetServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1_resources_pb.Service|null) => void
  ): UnaryResponse;
  getService(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.GetServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1_resources_pb.Service|null) => void
  ): UnaryResponse;
  listServices(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.ListServicesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1_serviceusage_pb.ListServicesResponse|null) => void
  ): UnaryResponse;
  listServices(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.ListServicesRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1_serviceusage_pb.ListServicesResponse|null) => void
  ): UnaryResponse;
  batchEnableServices(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.BatchEnableServicesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  batchEnableServices(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.BatchEnableServicesRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  batchGetServices(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesResponse|null) => void
  ): UnaryResponse;
  batchGetServices(
    requestMessage: google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesResponse|null) => void
  ): UnaryResponse;
}

