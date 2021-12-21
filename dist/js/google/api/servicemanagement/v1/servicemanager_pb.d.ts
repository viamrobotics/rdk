// package: google.api.servicemanagement.v1
// file: google/api/servicemanagement/v1/servicemanager.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_client_pb from "../../../../google/api/client_pb";
import * as google_api_field_behavior_pb from "../../../../google/api/field_behavior_pb";
import * as google_api_service_pb from "../../../../google/api/service_pb";
import * as google_api_servicemanagement_v1_resources_pb from "../../../../google/api/servicemanagement/v1/resources_pb";
import * as google_longrunning_operations_pb from "../../../../google/longrunning/operations_pb";
import * as google_protobuf_any_pb from "google-protobuf/google/protobuf/any_pb";
import * as google_protobuf_field_mask_pb from "google-protobuf/google/protobuf/field_mask_pb";
import * as google_rpc_status_pb from "../../../../google/rpc/status_pb";

export class ListServicesRequest extends jspb.Message {
  getProducerProjectId(): string;
  setProducerProjectId(value: string): void;

  getPageSize(): number;
  setPageSize(value: number): void;

  getPageToken(): string;
  setPageToken(value: string): void;

  getConsumerId(): string;
  setConsumerId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListServicesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListServicesRequest): ListServicesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListServicesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListServicesRequest;
  static deserializeBinaryFromReader(message: ListServicesRequest, reader: jspb.BinaryReader): ListServicesRequest;
}

export namespace ListServicesRequest {
  export type AsObject = {
    producerProjectId: string,
    pageSize: number,
    pageToken: string,
    consumerId: string,
  }
}

export class ListServicesResponse extends jspb.Message {
  clearServicesList(): void;
  getServicesList(): Array<google_api_servicemanagement_v1_resources_pb.ManagedService>;
  setServicesList(value: Array<google_api_servicemanagement_v1_resources_pb.ManagedService>): void;
  addServices(value?: google_api_servicemanagement_v1_resources_pb.ManagedService, index?: number): google_api_servicemanagement_v1_resources_pb.ManagedService;

  getNextPageToken(): string;
  setNextPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListServicesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListServicesResponse): ListServicesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListServicesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListServicesResponse;
  static deserializeBinaryFromReader(message: ListServicesResponse, reader: jspb.BinaryReader): ListServicesResponse;
}

export namespace ListServicesResponse {
  export type AsObject = {
    servicesList: Array<google_api_servicemanagement_v1_resources_pb.ManagedService.AsObject>,
    nextPageToken: string,
  }
}

export class GetServiceRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetServiceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetServiceRequest): GetServiceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetServiceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetServiceRequest;
  static deserializeBinaryFromReader(message: GetServiceRequest, reader: jspb.BinaryReader): GetServiceRequest;
}

export namespace GetServiceRequest {
  export type AsObject = {
    serviceName: string,
  }
}

export class CreateServiceRequest extends jspb.Message {
  hasService(): boolean;
  clearService(): void;
  getService(): google_api_servicemanagement_v1_resources_pb.ManagedService | undefined;
  setService(value?: google_api_servicemanagement_v1_resources_pb.ManagedService): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateServiceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateServiceRequest): CreateServiceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateServiceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateServiceRequest;
  static deserializeBinaryFromReader(message: CreateServiceRequest, reader: jspb.BinaryReader): CreateServiceRequest;
}

export namespace CreateServiceRequest {
  export type AsObject = {
    service?: google_api_servicemanagement_v1_resources_pb.ManagedService.AsObject,
  }
}

export class DeleteServiceRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeleteServiceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: DeleteServiceRequest): DeleteServiceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DeleteServiceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeleteServiceRequest;
  static deserializeBinaryFromReader(message: DeleteServiceRequest, reader: jspb.BinaryReader): DeleteServiceRequest;
}

export namespace DeleteServiceRequest {
  export type AsObject = {
    serviceName: string,
  }
}

export class UndeleteServiceRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UndeleteServiceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UndeleteServiceRequest): UndeleteServiceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UndeleteServiceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UndeleteServiceRequest;
  static deserializeBinaryFromReader(message: UndeleteServiceRequest, reader: jspb.BinaryReader): UndeleteServiceRequest;
}

export namespace UndeleteServiceRequest {
  export type AsObject = {
    serviceName: string,
  }
}

export class UndeleteServiceResponse extends jspb.Message {
  hasService(): boolean;
  clearService(): void;
  getService(): google_api_servicemanagement_v1_resources_pb.ManagedService | undefined;
  setService(value?: google_api_servicemanagement_v1_resources_pb.ManagedService): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UndeleteServiceResponse.AsObject;
  static toObject(includeInstance: boolean, msg: UndeleteServiceResponse): UndeleteServiceResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UndeleteServiceResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UndeleteServiceResponse;
  static deserializeBinaryFromReader(message: UndeleteServiceResponse, reader: jspb.BinaryReader): UndeleteServiceResponse;
}

export namespace UndeleteServiceResponse {
  export type AsObject = {
    service?: google_api_servicemanagement_v1_resources_pb.ManagedService.AsObject,
  }
}

export class GetServiceConfigRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  getConfigId(): string;
  setConfigId(value: string): void;

  getView(): GetServiceConfigRequest.ConfigViewMap[keyof GetServiceConfigRequest.ConfigViewMap];
  setView(value: GetServiceConfigRequest.ConfigViewMap[keyof GetServiceConfigRequest.ConfigViewMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetServiceConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetServiceConfigRequest): GetServiceConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetServiceConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetServiceConfigRequest;
  static deserializeBinaryFromReader(message: GetServiceConfigRequest, reader: jspb.BinaryReader): GetServiceConfigRequest;
}

export namespace GetServiceConfigRequest {
  export type AsObject = {
    serviceName: string,
    configId: string,
    view: GetServiceConfigRequest.ConfigViewMap[keyof GetServiceConfigRequest.ConfigViewMap],
  }

  export interface ConfigViewMap {
    BASIC: 0;
    FULL: 1;
  }

  export const ConfigView: ConfigViewMap;
}

export class ListServiceConfigsRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  getPageToken(): string;
  setPageToken(value: string): void;

  getPageSize(): number;
  setPageSize(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListServiceConfigsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListServiceConfigsRequest): ListServiceConfigsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListServiceConfigsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListServiceConfigsRequest;
  static deserializeBinaryFromReader(message: ListServiceConfigsRequest, reader: jspb.BinaryReader): ListServiceConfigsRequest;
}

export namespace ListServiceConfigsRequest {
  export type AsObject = {
    serviceName: string,
    pageToken: string,
    pageSize: number,
  }
}

export class ListServiceConfigsResponse extends jspb.Message {
  clearServiceConfigsList(): void;
  getServiceConfigsList(): Array<google_api_service_pb.Service>;
  setServiceConfigsList(value: Array<google_api_service_pb.Service>): void;
  addServiceConfigs(value?: google_api_service_pb.Service, index?: number): google_api_service_pb.Service;

  getNextPageToken(): string;
  setNextPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListServiceConfigsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListServiceConfigsResponse): ListServiceConfigsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListServiceConfigsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListServiceConfigsResponse;
  static deserializeBinaryFromReader(message: ListServiceConfigsResponse, reader: jspb.BinaryReader): ListServiceConfigsResponse;
}

export namespace ListServiceConfigsResponse {
  export type AsObject = {
    serviceConfigsList: Array<google_api_service_pb.Service.AsObject>,
    nextPageToken: string,
  }
}

export class CreateServiceConfigRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  hasServiceConfig(): boolean;
  clearServiceConfig(): void;
  getServiceConfig(): google_api_service_pb.Service | undefined;
  setServiceConfig(value?: google_api_service_pb.Service): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateServiceConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateServiceConfigRequest): CreateServiceConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateServiceConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateServiceConfigRequest;
  static deserializeBinaryFromReader(message: CreateServiceConfigRequest, reader: jspb.BinaryReader): CreateServiceConfigRequest;
}

export namespace CreateServiceConfigRequest {
  export type AsObject = {
    serviceName: string,
    serviceConfig?: google_api_service_pb.Service.AsObject,
  }
}

export class SubmitConfigSourceRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  hasConfigSource(): boolean;
  clearConfigSource(): void;
  getConfigSource(): google_api_servicemanagement_v1_resources_pb.ConfigSource | undefined;
  setConfigSource(value?: google_api_servicemanagement_v1_resources_pb.ConfigSource): void;

  getValidateOnly(): boolean;
  setValidateOnly(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubmitConfigSourceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SubmitConfigSourceRequest): SubmitConfigSourceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SubmitConfigSourceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SubmitConfigSourceRequest;
  static deserializeBinaryFromReader(message: SubmitConfigSourceRequest, reader: jspb.BinaryReader): SubmitConfigSourceRequest;
}

export namespace SubmitConfigSourceRequest {
  export type AsObject = {
    serviceName: string,
    configSource?: google_api_servicemanagement_v1_resources_pb.ConfigSource.AsObject,
    validateOnly: boolean,
  }
}

export class SubmitConfigSourceResponse extends jspb.Message {
  hasServiceConfig(): boolean;
  clearServiceConfig(): void;
  getServiceConfig(): google_api_service_pb.Service | undefined;
  setServiceConfig(value?: google_api_service_pb.Service): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SubmitConfigSourceResponse.AsObject;
  static toObject(includeInstance: boolean, msg: SubmitConfigSourceResponse): SubmitConfigSourceResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SubmitConfigSourceResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SubmitConfigSourceResponse;
  static deserializeBinaryFromReader(message: SubmitConfigSourceResponse, reader: jspb.BinaryReader): SubmitConfigSourceResponse;
}

export namespace SubmitConfigSourceResponse {
  export type AsObject = {
    serviceConfig?: google_api_service_pb.Service.AsObject,
  }
}

export class CreateServiceRolloutRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  hasRollout(): boolean;
  clearRollout(): void;
  getRollout(): google_api_servicemanagement_v1_resources_pb.Rollout | undefined;
  setRollout(value?: google_api_servicemanagement_v1_resources_pb.Rollout): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateServiceRolloutRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateServiceRolloutRequest): CreateServiceRolloutRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateServiceRolloutRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateServiceRolloutRequest;
  static deserializeBinaryFromReader(message: CreateServiceRolloutRequest, reader: jspb.BinaryReader): CreateServiceRolloutRequest;
}

export namespace CreateServiceRolloutRequest {
  export type AsObject = {
    serviceName: string,
    rollout?: google_api_servicemanagement_v1_resources_pb.Rollout.AsObject,
  }
}

export class ListServiceRolloutsRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  getPageToken(): string;
  setPageToken(value: string): void;

  getPageSize(): number;
  setPageSize(value: number): void;

  getFilter(): string;
  setFilter(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListServiceRolloutsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListServiceRolloutsRequest): ListServiceRolloutsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListServiceRolloutsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListServiceRolloutsRequest;
  static deserializeBinaryFromReader(message: ListServiceRolloutsRequest, reader: jspb.BinaryReader): ListServiceRolloutsRequest;
}

export namespace ListServiceRolloutsRequest {
  export type AsObject = {
    serviceName: string,
    pageToken: string,
    pageSize: number,
    filter: string,
  }
}

export class ListServiceRolloutsResponse extends jspb.Message {
  clearRolloutsList(): void;
  getRolloutsList(): Array<google_api_servicemanagement_v1_resources_pb.Rollout>;
  setRolloutsList(value: Array<google_api_servicemanagement_v1_resources_pb.Rollout>): void;
  addRollouts(value?: google_api_servicemanagement_v1_resources_pb.Rollout, index?: number): google_api_servicemanagement_v1_resources_pb.Rollout;

  getNextPageToken(): string;
  setNextPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListServiceRolloutsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListServiceRolloutsResponse): ListServiceRolloutsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListServiceRolloutsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListServiceRolloutsResponse;
  static deserializeBinaryFromReader(message: ListServiceRolloutsResponse, reader: jspb.BinaryReader): ListServiceRolloutsResponse;
}

export namespace ListServiceRolloutsResponse {
  export type AsObject = {
    rolloutsList: Array<google_api_servicemanagement_v1_resources_pb.Rollout.AsObject>,
    nextPageToken: string,
  }
}

export class GetServiceRolloutRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  getRolloutId(): string;
  setRolloutId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetServiceRolloutRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetServiceRolloutRequest): GetServiceRolloutRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetServiceRolloutRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetServiceRolloutRequest;
  static deserializeBinaryFromReader(message: GetServiceRolloutRequest, reader: jspb.BinaryReader): GetServiceRolloutRequest;
}

export namespace GetServiceRolloutRequest {
  export type AsObject = {
    serviceName: string,
    rolloutId: string,
  }
}

export class EnableServiceRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  getConsumerId(): string;
  setConsumerId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EnableServiceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: EnableServiceRequest): EnableServiceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EnableServiceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EnableServiceRequest;
  static deserializeBinaryFromReader(message: EnableServiceRequest, reader: jspb.BinaryReader): EnableServiceRequest;
}

export namespace EnableServiceRequest {
  export type AsObject = {
    serviceName: string,
    consumerId: string,
  }
}

export class EnableServiceResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EnableServiceResponse.AsObject;
  static toObject(includeInstance: boolean, msg: EnableServiceResponse): EnableServiceResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EnableServiceResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EnableServiceResponse;
  static deserializeBinaryFromReader(message: EnableServiceResponse, reader: jspb.BinaryReader): EnableServiceResponse;
}

export namespace EnableServiceResponse {
  export type AsObject = {
  }
}

export class DisableServiceRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  getConsumerId(): string;
  setConsumerId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DisableServiceRequest.AsObject;
  static toObject(includeInstance: boolean, msg: DisableServiceRequest): DisableServiceRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DisableServiceRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DisableServiceRequest;
  static deserializeBinaryFromReader(message: DisableServiceRequest, reader: jspb.BinaryReader): DisableServiceRequest;
}

export namespace DisableServiceRequest {
  export type AsObject = {
    serviceName: string,
    consumerId: string,
  }
}

export class DisableServiceResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DisableServiceResponse.AsObject;
  static toObject(includeInstance: boolean, msg: DisableServiceResponse): DisableServiceResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DisableServiceResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DisableServiceResponse;
  static deserializeBinaryFromReader(message: DisableServiceResponse, reader: jspb.BinaryReader): DisableServiceResponse;
}

export namespace DisableServiceResponse {
  export type AsObject = {
  }
}

export class GenerateConfigReportRequest extends jspb.Message {
  hasNewConfig(): boolean;
  clearNewConfig(): void;
  getNewConfig(): google_protobuf_any_pb.Any | undefined;
  setNewConfig(value?: google_protobuf_any_pb.Any): void;

  hasOldConfig(): boolean;
  clearOldConfig(): void;
  getOldConfig(): google_protobuf_any_pb.Any | undefined;
  setOldConfig(value?: google_protobuf_any_pb.Any): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GenerateConfigReportRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GenerateConfigReportRequest): GenerateConfigReportRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GenerateConfigReportRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GenerateConfigReportRequest;
  static deserializeBinaryFromReader(message: GenerateConfigReportRequest, reader: jspb.BinaryReader): GenerateConfigReportRequest;
}

export namespace GenerateConfigReportRequest {
  export type AsObject = {
    newConfig?: google_protobuf_any_pb.Any.AsObject,
    oldConfig?: google_protobuf_any_pb.Any.AsObject,
  }
}

export class GenerateConfigReportResponse extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  getId(): string;
  setId(value: string): void;

  clearChangeReportsList(): void;
  getChangeReportsList(): Array<google_api_servicemanagement_v1_resources_pb.ChangeReport>;
  setChangeReportsList(value: Array<google_api_servicemanagement_v1_resources_pb.ChangeReport>): void;
  addChangeReports(value?: google_api_servicemanagement_v1_resources_pb.ChangeReport, index?: number): google_api_servicemanagement_v1_resources_pb.ChangeReport;

  clearDiagnosticsList(): void;
  getDiagnosticsList(): Array<google_api_servicemanagement_v1_resources_pb.Diagnostic>;
  setDiagnosticsList(value: Array<google_api_servicemanagement_v1_resources_pb.Diagnostic>): void;
  addDiagnostics(value?: google_api_servicemanagement_v1_resources_pb.Diagnostic, index?: number): google_api_servicemanagement_v1_resources_pb.Diagnostic;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GenerateConfigReportResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GenerateConfigReportResponse): GenerateConfigReportResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GenerateConfigReportResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GenerateConfigReportResponse;
  static deserializeBinaryFromReader(message: GenerateConfigReportResponse, reader: jspb.BinaryReader): GenerateConfigReportResponse;
}

export namespace GenerateConfigReportResponse {
  export type AsObject = {
    serviceName: string,
    id: string,
    changeReportsList: Array<google_api_servicemanagement_v1_resources_pb.ChangeReport.AsObject>,
    diagnosticsList: Array<google_api_servicemanagement_v1_resources_pb.Diagnostic.AsObject>,
  }
}

