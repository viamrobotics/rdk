// package: google.api.serviceusage.v1beta1
// file: google/api/serviceusage/v1beta1/serviceusage.proto

import * as google_api_serviceusage_v1beta1_serviceusage_pb from "../../../../google/api/serviceusage/v1beta1/serviceusage_pb";
import * as google_api_serviceusage_v1beta1_resources_pb from "../../../../google/api/serviceusage/v1beta1/resources_pb";
import * as google_longrunning_operations_pb from "../../../../google/longrunning/operations_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ServiceUsageEnableService = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.EnableServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageDisableService = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.DisableServiceRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageGetService = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.GetServiceRequest;
  readonly responseType: typeof google_api_serviceusage_v1beta1_resources_pb.Service;
};

type ServiceUsageListServices = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesRequest;
  readonly responseType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesResponse;
};

type ServiceUsageBatchEnableServices = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.BatchEnableServicesRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageListConsumerQuotaMetrics = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsRequest;
  readonly responseType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsResponse;
};

type ServiceUsageGetConsumerQuotaMetric = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaMetricRequest;
  readonly responseType: typeof google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric;
};

type ServiceUsageGetConsumerQuotaLimit = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaLimitRequest;
  readonly responseType: typeof google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaLimit;
};

type ServiceUsageCreateAdminOverride = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.CreateAdminOverrideRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageUpdateAdminOverride = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.UpdateAdminOverrideRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageDeleteAdminOverride = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.DeleteAdminOverrideRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageListAdminOverrides = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesRequest;
  readonly responseType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesResponse;
};

type ServiceUsageImportAdminOverrides = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ImportAdminOverridesRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageCreateConsumerOverride = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.CreateConsumerOverrideRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageUpdateConsumerOverride = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.UpdateConsumerOverrideRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageDeleteConsumerOverride = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.DeleteConsumerOverrideRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageListConsumerOverrides = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesRequest;
  readonly responseType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesResponse;
};

type ServiceUsageImportConsumerOverrides = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.ImportConsumerOverridesRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

type ServiceUsageGenerateServiceIdentity = {
  readonly methodName: string;
  readonly service: typeof ServiceUsage;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_serviceusage_v1beta1_serviceusage_pb.GenerateServiceIdentityRequest;
  readonly responseType: typeof google_longrunning_operations_pb.Operation;
};

export class ServiceUsage {
  static readonly serviceName: string;
  static readonly EnableService: ServiceUsageEnableService;
  static readonly DisableService: ServiceUsageDisableService;
  static readonly GetService: ServiceUsageGetService;
  static readonly ListServices: ServiceUsageListServices;
  static readonly BatchEnableServices: ServiceUsageBatchEnableServices;
  static readonly ListConsumerQuotaMetrics: ServiceUsageListConsumerQuotaMetrics;
  static readonly GetConsumerQuotaMetric: ServiceUsageGetConsumerQuotaMetric;
  static readonly GetConsumerQuotaLimit: ServiceUsageGetConsumerQuotaLimit;
  static readonly CreateAdminOverride: ServiceUsageCreateAdminOverride;
  static readonly UpdateAdminOverride: ServiceUsageUpdateAdminOverride;
  static readonly DeleteAdminOverride: ServiceUsageDeleteAdminOverride;
  static readonly ListAdminOverrides: ServiceUsageListAdminOverrides;
  static readonly ImportAdminOverrides: ServiceUsageImportAdminOverrides;
  static readonly CreateConsumerOverride: ServiceUsageCreateConsumerOverride;
  static readonly UpdateConsumerOverride: ServiceUsageUpdateConsumerOverride;
  static readonly DeleteConsumerOverride: ServiceUsageDeleteConsumerOverride;
  static readonly ListConsumerOverrides: ServiceUsageListConsumerOverrides;
  static readonly ImportConsumerOverrides: ServiceUsageImportConsumerOverrides;
  static readonly GenerateServiceIdentity: ServiceUsageGenerateServiceIdentity;
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
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.EnableServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  enableService(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.EnableServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  disableService(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.DisableServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  disableService(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.DisableServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  getService(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GetServiceRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_resources_pb.Service|null) => void
  ): UnaryResponse;
  getService(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GetServiceRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_resources_pb.Service|null) => void
  ): UnaryResponse;
  listServices(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesResponse|null) => void
  ): UnaryResponse;
  listServices(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesResponse|null) => void
  ): UnaryResponse;
  batchEnableServices(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.BatchEnableServicesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  batchEnableServices(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.BatchEnableServicesRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  listConsumerQuotaMetrics(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsResponse|null) => void
  ): UnaryResponse;
  listConsumerQuotaMetrics(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsResponse|null) => void
  ): UnaryResponse;
  getConsumerQuotaMetric(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaMetricRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric|null) => void
  ): UnaryResponse;
  getConsumerQuotaMetric(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaMetricRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric|null) => void
  ): UnaryResponse;
  getConsumerQuotaLimit(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaLimitRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaLimit|null) => void
  ): UnaryResponse;
  getConsumerQuotaLimit(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaLimitRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaLimit|null) => void
  ): UnaryResponse;
  createAdminOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.CreateAdminOverrideRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  createAdminOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.CreateAdminOverrideRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  updateAdminOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.UpdateAdminOverrideRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  updateAdminOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.UpdateAdminOverrideRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  deleteAdminOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.DeleteAdminOverrideRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  deleteAdminOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.DeleteAdminOverrideRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  listAdminOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesResponse|null) => void
  ): UnaryResponse;
  listAdminOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesResponse|null) => void
  ): UnaryResponse;
  importAdminOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ImportAdminOverridesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  importAdminOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ImportAdminOverridesRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  createConsumerOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.CreateConsumerOverrideRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  createConsumerOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.CreateConsumerOverrideRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  updateConsumerOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.UpdateConsumerOverrideRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  updateConsumerOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.UpdateConsumerOverrideRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  deleteConsumerOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.DeleteConsumerOverrideRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  deleteConsumerOverride(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.DeleteConsumerOverrideRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  listConsumerOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesResponse|null) => void
  ): UnaryResponse;
  listConsumerOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesResponse|null) => void
  ): UnaryResponse;
  importConsumerOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ImportConsumerOverridesRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  importConsumerOverrides(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.ImportConsumerOverridesRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  generateServiceIdentity(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GenerateServiceIdentityRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
  generateServiceIdentity(
    requestMessage: google_api_serviceusage_v1beta1_serviceusage_pb.GenerateServiceIdentityRequest,
    callback: (error: ServiceError|null, responseMessage: google_longrunning_operations_pb.Operation|null) => void
  ): UnaryResponse;
}

