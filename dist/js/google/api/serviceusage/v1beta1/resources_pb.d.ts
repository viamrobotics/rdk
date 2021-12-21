// package: google.api.serviceusage.v1beta1
// file: google/api/serviceusage/v1beta1/resources.proto

import * as jspb from "google-protobuf";
import * as google_api_auth_pb from "../../../../google/api/auth_pb";
import * as google_api_documentation_pb from "../../../../google/api/documentation_pb";
import * as google_api_endpoint_pb from "../../../../google/api/endpoint_pb";
import * as google_api_monitored_resource_pb from "../../../../google/api/monitored_resource_pb";
import * as google_api_monitoring_pb from "../../../../google/api/monitoring_pb";
import * as google_api_quota_pb from "../../../../google/api/quota_pb";
import * as google_api_usage_pb from "../../../../google/api/usage_pb";
import * as google_protobuf_api_pb from "google-protobuf/google/protobuf/api_pb";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class Service extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getParent(): string;
  setParent(value: string): void;

  hasConfig(): boolean;
  clearConfig(): void;
  getConfig(): ServiceConfig | undefined;
  setConfig(value?: ServiceConfig): void;

  getState(): StateMap[keyof StateMap];
  setState(value: StateMap[keyof StateMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Service.AsObject;
  static toObject(includeInstance: boolean, msg: Service): Service.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Service, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Service;
  static deserializeBinaryFromReader(message: Service, reader: jspb.BinaryReader): Service;
}

export namespace Service {
  export type AsObject = {
    name: string,
    parent: string,
    config?: ServiceConfig.AsObject,
    state: StateMap[keyof StateMap],
  }
}

export class ServiceConfig extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getTitle(): string;
  setTitle(value: string): void;

  clearApisList(): void;
  getApisList(): Array<google_protobuf_api_pb.Api>;
  setApisList(value: Array<google_protobuf_api_pb.Api>): void;
  addApis(value?: google_protobuf_api_pb.Api, index?: number): google_protobuf_api_pb.Api;

  hasDocumentation(): boolean;
  clearDocumentation(): void;
  getDocumentation(): google_api_documentation_pb.Documentation | undefined;
  setDocumentation(value?: google_api_documentation_pb.Documentation): void;

  hasQuota(): boolean;
  clearQuota(): void;
  getQuota(): google_api_quota_pb.Quota | undefined;
  setQuota(value?: google_api_quota_pb.Quota): void;

  hasAuthentication(): boolean;
  clearAuthentication(): void;
  getAuthentication(): google_api_auth_pb.Authentication | undefined;
  setAuthentication(value?: google_api_auth_pb.Authentication): void;

  hasUsage(): boolean;
  clearUsage(): void;
  getUsage(): google_api_usage_pb.Usage | undefined;
  setUsage(value?: google_api_usage_pb.Usage): void;

  clearEndpointsList(): void;
  getEndpointsList(): Array<google_api_endpoint_pb.Endpoint>;
  setEndpointsList(value: Array<google_api_endpoint_pb.Endpoint>): void;
  addEndpoints(value?: google_api_endpoint_pb.Endpoint, index?: number): google_api_endpoint_pb.Endpoint;

  clearMonitoredResourcesList(): void;
  getMonitoredResourcesList(): Array<google_api_monitored_resource_pb.MonitoredResourceDescriptor>;
  setMonitoredResourcesList(value: Array<google_api_monitored_resource_pb.MonitoredResourceDescriptor>): void;
  addMonitoredResources(value?: google_api_monitored_resource_pb.MonitoredResourceDescriptor, index?: number): google_api_monitored_resource_pb.MonitoredResourceDescriptor;

  hasMonitoring(): boolean;
  clearMonitoring(): void;
  getMonitoring(): google_api_monitoring_pb.Monitoring | undefined;
  setMonitoring(value?: google_api_monitoring_pb.Monitoring): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServiceConfig.AsObject;
  static toObject(includeInstance: boolean, msg: ServiceConfig): ServiceConfig.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServiceConfig, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServiceConfig;
  static deserializeBinaryFromReader(message: ServiceConfig, reader: jspb.BinaryReader): ServiceConfig;
}

export namespace ServiceConfig {
  export type AsObject = {
    name: string,
    title: string,
    apisList: Array<google_protobuf_api_pb.Api.AsObject>,
    documentation?: google_api_documentation_pb.Documentation.AsObject,
    quota?: google_api_quota_pb.Quota.AsObject,
    authentication?: google_api_auth_pb.Authentication.AsObject,
    usage?: google_api_usage_pb.Usage.AsObject,
    endpointsList: Array<google_api_endpoint_pb.Endpoint.AsObject>,
    monitoredResourcesList: Array<google_api_monitored_resource_pb.MonitoredResourceDescriptor.AsObject>,
    monitoring?: google_api_monitoring_pb.Monitoring.AsObject,
  }
}

export class OperationMetadata extends jspb.Message {
  clearResourceNamesList(): void;
  getResourceNamesList(): Array<string>;
  setResourceNamesList(value: Array<string>): void;
  addResourceNames(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): OperationMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: OperationMetadata): OperationMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: OperationMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): OperationMetadata;
  static deserializeBinaryFromReader(message: OperationMetadata, reader: jspb.BinaryReader): OperationMetadata;
}

export namespace OperationMetadata {
  export type AsObject = {
    resourceNamesList: Array<string>,
  }
}

export class ConsumerQuotaMetric extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMetric(): string;
  setMetric(value: string): void;

  getDisplayName(): string;
  setDisplayName(value: string): void;

  clearConsumerQuotaLimitsList(): void;
  getConsumerQuotaLimitsList(): Array<ConsumerQuotaLimit>;
  setConsumerQuotaLimitsList(value: Array<ConsumerQuotaLimit>): void;
  addConsumerQuotaLimits(value?: ConsumerQuotaLimit, index?: number): ConsumerQuotaLimit;

  clearDescendantConsumerQuotaLimitsList(): void;
  getDescendantConsumerQuotaLimitsList(): Array<ConsumerQuotaLimit>;
  setDescendantConsumerQuotaLimitsList(value: Array<ConsumerQuotaLimit>): void;
  addDescendantConsumerQuotaLimits(value?: ConsumerQuotaLimit, index?: number): ConsumerQuotaLimit;

  getUnit(): string;
  setUnit(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ConsumerQuotaMetric.AsObject;
  static toObject(includeInstance: boolean, msg: ConsumerQuotaMetric): ConsumerQuotaMetric.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ConsumerQuotaMetric, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ConsumerQuotaMetric;
  static deserializeBinaryFromReader(message: ConsumerQuotaMetric, reader: jspb.BinaryReader): ConsumerQuotaMetric;
}

export namespace ConsumerQuotaMetric {
  export type AsObject = {
    name: string,
    metric: string,
    displayName: string,
    consumerQuotaLimitsList: Array<ConsumerQuotaLimit.AsObject>,
    descendantConsumerQuotaLimitsList: Array<ConsumerQuotaLimit.AsObject>,
    unit: string,
  }
}

export class ConsumerQuotaLimit extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMetric(): string;
  setMetric(value: string): void;

  getUnit(): string;
  setUnit(value: string): void;

  getIsPrecise(): boolean;
  setIsPrecise(value: boolean): void;

  getAllowsAdminOverrides(): boolean;
  setAllowsAdminOverrides(value: boolean): void;

  clearQuotaBucketsList(): void;
  getQuotaBucketsList(): Array<QuotaBucket>;
  setQuotaBucketsList(value: Array<QuotaBucket>): void;
  addQuotaBuckets(value?: QuotaBucket, index?: number): QuotaBucket;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ConsumerQuotaLimit.AsObject;
  static toObject(includeInstance: boolean, msg: ConsumerQuotaLimit): ConsumerQuotaLimit.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ConsumerQuotaLimit, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ConsumerQuotaLimit;
  static deserializeBinaryFromReader(message: ConsumerQuotaLimit, reader: jspb.BinaryReader): ConsumerQuotaLimit;
}

export namespace ConsumerQuotaLimit {
  export type AsObject = {
    name: string,
    metric: string,
    unit: string,
    isPrecise: boolean,
    allowsAdminOverrides: boolean,
    quotaBucketsList: Array<QuotaBucket.AsObject>,
  }
}

export class QuotaBucket extends jspb.Message {
  getEffectiveLimit(): number;
  setEffectiveLimit(value: number): void;

  getDefaultLimit(): number;
  setDefaultLimit(value: number): void;

  hasProducerOverride(): boolean;
  clearProducerOverride(): void;
  getProducerOverride(): QuotaOverride | undefined;
  setProducerOverride(value?: QuotaOverride): void;

  hasConsumerOverride(): boolean;
  clearConsumerOverride(): void;
  getConsumerOverride(): QuotaOverride | undefined;
  setConsumerOverride(value?: QuotaOverride): void;

  hasAdminOverride(): boolean;
  clearAdminOverride(): void;
  getAdminOverride(): QuotaOverride | undefined;
  setAdminOverride(value?: QuotaOverride): void;

  getDimensionsMap(): jspb.Map<string, string>;
  clearDimensionsMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): QuotaBucket.AsObject;
  static toObject(includeInstance: boolean, msg: QuotaBucket): QuotaBucket.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: QuotaBucket, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): QuotaBucket;
  static deserializeBinaryFromReader(message: QuotaBucket, reader: jspb.BinaryReader): QuotaBucket;
}

export namespace QuotaBucket {
  export type AsObject = {
    effectiveLimit: number,
    defaultLimit: number,
    producerOverride?: QuotaOverride.AsObject,
    consumerOverride?: QuotaOverride.AsObject,
    adminOverride?: QuotaOverride.AsObject,
    dimensionsMap: Array<[string, string]>,
  }
}

export class QuotaOverride extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getOverrideValue(): number;
  setOverrideValue(value: number): void;

  getDimensionsMap(): jspb.Map<string, string>;
  clearDimensionsMap(): void;
  getMetric(): string;
  setMetric(value: string): void;

  getUnit(): string;
  setUnit(value: string): void;

  getAdminOverrideAncestor(): string;
  setAdminOverrideAncestor(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): QuotaOverride.AsObject;
  static toObject(includeInstance: boolean, msg: QuotaOverride): QuotaOverride.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: QuotaOverride, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): QuotaOverride;
  static deserializeBinaryFromReader(message: QuotaOverride, reader: jspb.BinaryReader): QuotaOverride;
}

export namespace QuotaOverride {
  export type AsObject = {
    name: string,
    overrideValue: number,
    dimensionsMap: Array<[string, string]>,
    metric: string,
    unit: string,
    adminOverrideAncestor: string,
  }
}

export class OverrideInlineSource extends jspb.Message {
  clearOverridesList(): void;
  getOverridesList(): Array<QuotaOverride>;
  setOverridesList(value: Array<QuotaOverride>): void;
  addOverrides(value?: QuotaOverride, index?: number): QuotaOverride;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): OverrideInlineSource.AsObject;
  static toObject(includeInstance: boolean, msg: OverrideInlineSource): OverrideInlineSource.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: OverrideInlineSource, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): OverrideInlineSource;
  static deserializeBinaryFromReader(message: OverrideInlineSource, reader: jspb.BinaryReader): OverrideInlineSource;
}

export namespace OverrideInlineSource {
  export type AsObject = {
    overridesList: Array<QuotaOverride.AsObject>,
  }
}

export class AdminQuotaPolicy extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPolicyValue(): number;
  setPolicyValue(value: number): void;

  getDimensionsMap(): jspb.Map<string, string>;
  clearDimensionsMap(): void;
  getMetric(): string;
  setMetric(value: string): void;

  getUnit(): string;
  setUnit(value: string): void;

  getContainer(): string;
  setContainer(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AdminQuotaPolicy.AsObject;
  static toObject(includeInstance: boolean, msg: AdminQuotaPolicy): AdminQuotaPolicy.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AdminQuotaPolicy, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AdminQuotaPolicy;
  static deserializeBinaryFromReader(message: AdminQuotaPolicy, reader: jspb.BinaryReader): AdminQuotaPolicy;
}

export namespace AdminQuotaPolicy {
  export type AsObject = {
    name: string,
    policyValue: number,
    dimensionsMap: Array<[string, string]>,
    metric: string,
    unit: string,
    container: string,
  }
}

export class ServiceIdentity extends jspb.Message {
  getEmail(): string;
  setEmail(value: string): void;

  getUniqueId(): string;
  setUniqueId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServiceIdentity.AsObject;
  static toObject(includeInstance: boolean, msg: ServiceIdentity): ServiceIdentity.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServiceIdentity, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServiceIdentity;
  static deserializeBinaryFromReader(message: ServiceIdentity, reader: jspb.BinaryReader): ServiceIdentity;
}

export namespace ServiceIdentity {
  export type AsObject = {
    email: string,
    uniqueId: string,
  }
}

export interface StateMap {
  STATE_UNSPECIFIED: 0;
  DISABLED: 1;
  ENABLED: 2;
}

export const State: StateMap;

export interface QuotaViewMap {
  QUOTA_VIEW_UNSPECIFIED: 0;
  BASIC: 1;
  FULL: 2;
}

export const QuotaView: QuotaViewMap;

export interface QuotaSafetyCheckMap {
  QUOTA_SAFETY_CHECK_UNSPECIFIED: 0;
  LIMIT_DECREASE_BELOW_USAGE: 1;
  LIMIT_DECREASE_PERCENTAGE_TOO_HIGH: 2;
}

export const QuotaSafetyCheck: QuotaSafetyCheckMap;

