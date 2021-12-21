// package: google.api.serviceusage.v1
// file: google/api/serviceusage/v1/resources.proto

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

export interface StateMap {
  STATE_UNSPECIFIED: 0;
  DISABLED: 1;
  ENABLED: 2;
}

export const State: StateMap;

