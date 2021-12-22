// package: google.api.servicemanagement.v1
// file: google/api/servicemanagement/v1/servicemanager.proto

import * as google_api_servicemanagement_v1_servicemanager_pb from "../../../../google/api/servicemanagement/v1/servicemanager_pb";
import * as google_api_service_pb from "../../../../google/api/service_pb";
import * as google_api_servicemanagement_v1_resources_pb from "../../../../google/api/servicemanagement/v1/resources_pb";
import * as google_longrunning_operations_pb from "../../../../google/longrunning/operations_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ServiceManagerListServices = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.ListServicesRequest;
  readonly responseType: typeof google_api_servicemanagement_v1_servicemanager_pb.ListServicesResponse;
};

type ServiceManagerGetService = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.GetServiceRequest;
  readonly responseType: typeof google_api_servicemanagement_v1_resources_pb.ManagedService;
};

type ServiceManagerCreateService = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceManagerDeleteService = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.DeleteServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceManagerUndeleteService = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.UndeleteServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceManagerListServiceConfigs = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsRequest;
  readonly responseType: typeof google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsResponse;
};

type ServiceManagerGetServiceConfig = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.GetServiceConfigRequest;
  readonly responseType: typeof google_api_service_pb.Service;
};

type ServiceManagerCreateServiceConfig = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.CreateServiceConfigRequest;
  readonly responseType: typeof google_api_service_pb.Service;
};

type ServiceManagerSubmitConfigSource = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.SubmitConfigSourceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceManagerListServiceRollouts = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsRequest;
  readonly responseType: typeof google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsResponse;
};

type ServiceManagerGetServiceRollout = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.GetServiceRolloutRequest;
  readonly responseType: typeof google_api_servicemanagement_v1_resources_pb.Rollout;
};

type ServiceManagerCreateServiceRollout = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRolloutRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceManagerGenerateConfigReport = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportRequest;
  readonly responseType: typeof google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportResponse;
};

type ServiceManagerEnableService = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.EnableServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceManagerDisableService = {
  readonly methodName: string;
  readonly service: typeof ServiceManager;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicemanagement_v1_servicemanager_pb.DisableServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

export class ServiceManager {
  static readonly serviceName: string;
  static readonly ListServices: ServiceManagerListServices;
  static readonly GetService: ServiceManagerGetService;
  static readonly CreateService: ServiceManagerCreateService;
  static readonly DeleteService: ServiceManagerDeleteService;
  static readonly UndeleteService: ServiceManagerUndeleteService;
  static readonly ListServiceConfigs: ServiceManagerListServiceConfigs;
  static readonly GetServiceConfig: ServiceManagerGetServiceConfig;
  static readonly CreateServiceConfig: ServiceManagerCreateServiceConfig;
  static readonly SubmitConfigSource: ServiceManagerSubmitConfigSource;
  static readonly ListServiceRollouts: ServiceManagerListServiceRollouts;
  static readonly GetServiceRollout: ServiceManagerGetServiceRollout;
  static readonly CreateServiceRollout: ServiceManagerCreateServiceRollout;
  static readonly GenerateConfigReport: ServiceManagerGenerateConfigReport;
  static readonly EnableService: ServiceManagerEnableService;
  static readonly DisableService: ServiceManagerDisableService;
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

export class ServiceManagerClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  listServices(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServicesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServicesResponse|null) => void
  ): UnaryResponse;
  listServices(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServicesRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServicesResponse|null) => void
  ): UnaryResponse;
  getService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GetServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_resources_pb.ManagedService|null) => void
  ): UnaryResponse;
  getService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GetServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_resources_pb.ManagedService|null) => void
  ): UnaryResponse;
  createService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  createService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  deleteService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.DeleteServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  deleteService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.DeleteServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  undeleteService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.UndeleteServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  undeleteService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.UndeleteServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  listServiceConfigs(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsResponse|null) => void
  ): UnaryResponse;
  listServiceConfigs(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsResponse|null) => void
  ): UnaryResponse;
  getServiceConfig(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GetServiceConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_service_pb.Service|null) => void
  ): UnaryResponse;
  getServiceConfig(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GetServiceConfigRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_service_pb.Service|null) => void
  ): UnaryResponse;
  createServiceConfig(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_service_pb.Service|null) => void
  ): UnaryResponse;
  createServiceConfig(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceConfigRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_service_pb.Service|null) => void
  ): UnaryResponse;
  submitConfigSource(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.SubmitConfigSourceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  submitConfigSource(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.SubmitConfigSourceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  listServiceRollouts(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsResponse|null) => void
  ): UnaryResponse;
  listServiceRollouts(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsResponse|null) => void
  ): UnaryResponse;
  getServiceRollout(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GetServiceRolloutRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_resources_pb.Rollout|null) => void
  ): UnaryResponse;
  getServiceRollout(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GetServiceRolloutRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_resources_pb.Rollout|null) => void
  ): UnaryResponse;
  createServiceRollout(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRolloutRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  createServiceRollout(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRolloutRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  generateConfigReport(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportResponse|null) => void
  ): UnaryResponse;
  generateConfigReport(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportResponse|null) => void
  ): UnaryResponse;
  enableService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.EnableServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  enableService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.EnableServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  disableService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.DisableServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  disableService(
    requestMessage: google_api_servicemanagement_v1_servicemanager_pb.DisableServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
}

