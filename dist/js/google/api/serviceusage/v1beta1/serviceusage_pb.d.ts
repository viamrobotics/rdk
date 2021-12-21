// package: google.api.serviceusage.v1beta1
// file: google/api/serviceusage/v1beta1/serviceusage.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_client_pb from "../../../../google/api/client_pb";
import * as google_api_field_behavior_pb from "../../../../google/api/field_behavior_pb";
import * as google_api_serviceusage_v1beta1_resources_pb from "../../../../google/api/serviceusage/v1beta1/resources_pb";
import * as google_longrunning_operations_pb from "../../../../google/longrunning/operations_pb";
import * as google_protobuf_field_mask_pb from "google-protobuf/google/protobuf/field_mask_pb";

export class EnableServiceRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

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
    name: string,
  }
}

export class DisableServiceRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

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
    name: string,
  }
}

export class GetServiceRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

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
    name: string,
  }
}

export class ListServicesRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  getPageSize(): number;
  setPageSize(value: number): void;

  getPageToken(): string;
  setPageToken(value: string): void;

  getFilter(): string;
  setFilter(value: string): void;

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
    parent: string,
    pageSize: number,
    pageToken: string,
    filter: string,
  }
}

export class ListServicesResponse extends jspb.Message {
  clearServicesList(): void;
  getServicesList(): Array<google_api_serviceusage_v1beta1_resources_pb.Service>;
  setServicesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.Service>): void;
  addServices(value?: google_api_serviceusage_v1beta1_resources_pb.Service, index?: number): google_api_serviceusage_v1beta1_resources_pb.Service;

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
    servicesList: Array<google_api_serviceusage_v1beta1_resources_pb.Service.AsObject>,
    nextPageToken: string,
  }
}

export class BatchEnableServicesRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  clearServiceIdsList(): void;
  getServiceIdsList(): Array<string>;
  setServiceIdsList(value: Array<string>): void;
  addServiceIds(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BatchEnableServicesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BatchEnableServicesRequest): BatchEnableServicesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BatchEnableServicesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BatchEnableServicesRequest;
  static deserializeBinaryFromReader(message: BatchEnableServicesRequest, reader: jspb.BinaryReader): BatchEnableServicesRequest;
}

export namespace BatchEnableServicesRequest {
  export type AsObject = {
    parent: string,
    serviceIdsList: Array<string>,
  }
}

export class ListConsumerQuotaMetricsRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  getPageSize(): number;
  setPageSize(value: number): void;

  getPageToken(): string;
  setPageToken(value: string): void;

  getView(): google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap];
  setView(value: google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListConsumerQuotaMetricsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListConsumerQuotaMetricsRequest): ListConsumerQuotaMetricsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListConsumerQuotaMetricsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListConsumerQuotaMetricsRequest;
  static deserializeBinaryFromReader(message: ListConsumerQuotaMetricsRequest, reader: jspb.BinaryReader): ListConsumerQuotaMetricsRequest;
}

export namespace ListConsumerQuotaMetricsRequest {
  export type AsObject = {
    parent: string,
    pageSize: number,
    pageToken: string,
    view: google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap],
  }
}

export class ListConsumerQuotaMetricsResponse extends jspb.Message {
  clearMetricsList(): void;
  getMetricsList(): Array<google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric>;
  setMetricsList(value: Array<google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric>): void;
  addMetrics(value?: google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric, index?: number): google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric;

  getNextPageToken(): string;
  setNextPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListConsumerQuotaMetricsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListConsumerQuotaMetricsResponse): ListConsumerQuotaMetricsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListConsumerQuotaMetricsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListConsumerQuotaMetricsResponse;
  static deserializeBinaryFromReader(message: ListConsumerQuotaMetricsResponse, reader: jspb.BinaryReader): ListConsumerQuotaMetricsResponse;
}

export namespace ListConsumerQuotaMetricsResponse {
  export type AsObject = {
    metricsList: Array<google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric.AsObject>,
    nextPageToken: string,
  }
}

export class GetConsumerQuotaMetricRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getView(): google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap];
  setView(value: google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetConsumerQuotaMetricRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetConsumerQuotaMetricRequest): GetConsumerQuotaMetricRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetConsumerQuotaMetricRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetConsumerQuotaMetricRequest;
  static deserializeBinaryFromReader(message: GetConsumerQuotaMetricRequest, reader: jspb.BinaryReader): GetConsumerQuotaMetricRequest;
}

export namespace GetConsumerQuotaMetricRequest {
  export type AsObject = {
    name: string,
    view: google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap],
  }
}

export class GetConsumerQuotaLimitRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getView(): google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap];
  setView(value: google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetConsumerQuotaLimitRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GetConsumerQuotaLimitRequest): GetConsumerQuotaLimitRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetConsumerQuotaLimitRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetConsumerQuotaLimitRequest;
  static deserializeBinaryFromReader(message: GetConsumerQuotaLimitRequest, reader: jspb.BinaryReader): GetConsumerQuotaLimitRequest;
}

export namespace GetConsumerQuotaLimitRequest {
  export type AsObject = {
    name: string,
    view: google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaViewMap],
  }
}

export class CreateAdminOverrideRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  hasOverride(): boolean;
  clearOverride(): void;
  getOverride(): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride | undefined;
  setOverride(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateAdminOverrideRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateAdminOverrideRequest): CreateAdminOverrideRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateAdminOverrideRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateAdminOverrideRequest;
  static deserializeBinaryFromReader(message: CreateAdminOverrideRequest, reader: jspb.BinaryReader): CreateAdminOverrideRequest;
}

export namespace CreateAdminOverrideRequest {
  export type AsObject = {
    parent: string,
    override?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject,
    force: boolean,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }
}

export class UpdateAdminOverrideRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasOverride(): boolean;
  clearOverride(): void;
  getOverride(): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride | undefined;
  setOverride(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  hasUpdateMask(): boolean;
  clearUpdateMask(): void;
  getUpdateMask(): google_protobuf_field_mask_pb.FieldMask | undefined;
  setUpdateMask(value?: google_protobuf_field_mask_pb.FieldMask): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateAdminOverrideRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateAdminOverrideRequest): UpdateAdminOverrideRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UpdateAdminOverrideRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateAdminOverrideRequest;
  static deserializeBinaryFromReader(message: UpdateAdminOverrideRequest, reader: jspb.BinaryReader): UpdateAdminOverrideRequest;
}

export namespace UpdateAdminOverrideRequest {
  export type AsObject = {
    name: string,
    override?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject,
    force: boolean,
    updateMask?: google_protobuf_field_mask_pb.FieldMask.AsObject,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }
}

export class DeleteAdminOverrideRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeleteAdminOverrideRequest.AsObject;
  static toObject(includeInstance: boolean, msg: DeleteAdminOverrideRequest): DeleteAdminOverrideRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DeleteAdminOverrideRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeleteAdminOverrideRequest;
  static deserializeBinaryFromReader(message: DeleteAdminOverrideRequest, reader: jspb.BinaryReader): DeleteAdminOverrideRequest;
}

export namespace DeleteAdminOverrideRequest {
  export type AsObject = {
    name: string,
    force: boolean,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }
}

export class ListAdminOverridesRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  getPageSize(): number;
  setPageSize(value: number): void;

  getPageToken(): string;
  setPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListAdminOverridesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListAdminOverridesRequest): ListAdminOverridesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListAdminOverridesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListAdminOverridesRequest;
  static deserializeBinaryFromReader(message: ListAdminOverridesRequest, reader: jspb.BinaryReader): ListAdminOverridesRequest;
}

export namespace ListAdminOverridesRequest {
  export type AsObject = {
    parent: string,
    pageSize: number,
    pageToken: string,
  }
}

export class ListAdminOverridesResponse extends jspb.Message {
  clearOverridesList(): void;
  getOverridesList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>;
  setOverridesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>): void;
  addOverrides(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;

  getNextPageToken(): string;
  setNextPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListAdminOverridesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListAdminOverridesResponse): ListAdminOverridesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListAdminOverridesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListAdminOverridesResponse;
  static deserializeBinaryFromReader(message: ListAdminOverridesResponse, reader: jspb.BinaryReader): ListAdminOverridesResponse;
}

export namespace ListAdminOverridesResponse {
  export type AsObject = {
    overridesList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject>,
    nextPageToken: string,
  }
}

export class BatchCreateAdminOverridesResponse extends jspb.Message {
  clearOverridesList(): void;
  getOverridesList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>;
  setOverridesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>): void;
  addOverrides(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BatchCreateAdminOverridesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BatchCreateAdminOverridesResponse): BatchCreateAdminOverridesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BatchCreateAdminOverridesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BatchCreateAdminOverridesResponse;
  static deserializeBinaryFromReader(message: BatchCreateAdminOverridesResponse, reader: jspb.BinaryReader): BatchCreateAdminOverridesResponse;
}

export namespace BatchCreateAdminOverridesResponse {
  export type AsObject = {
    overridesList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject>,
  }
}

export class ImportAdminOverridesRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  hasInlineSource(): boolean;
  clearInlineSource(): void;
  getInlineSource(): google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource | undefined;
  setInlineSource(value?: google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  getSourceCase(): ImportAdminOverridesRequest.SourceCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportAdminOverridesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ImportAdminOverridesRequest): ImportAdminOverridesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportAdminOverridesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportAdminOverridesRequest;
  static deserializeBinaryFromReader(message: ImportAdminOverridesRequest, reader: jspb.BinaryReader): ImportAdminOverridesRequest;
}

export namespace ImportAdminOverridesRequest {
  export type AsObject = {
    parent: string,
    inlineSource?: google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.AsObject,
    force: boolean,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }

  export enum SourceCase {
    SOURCE_NOT_SET = 0,
    INLINE_SOURCE = 2,
  }
}

export class ImportAdminOverridesResponse extends jspb.Message {
  clearOverridesList(): void;
  getOverridesList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>;
  setOverridesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>): void;
  addOverrides(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportAdminOverridesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ImportAdminOverridesResponse): ImportAdminOverridesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportAdminOverridesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportAdminOverridesResponse;
  static deserializeBinaryFromReader(message: ImportAdminOverridesResponse, reader: jspb.BinaryReader): ImportAdminOverridesResponse;
}

export namespace ImportAdminOverridesResponse {
  export type AsObject = {
    overridesList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject>,
  }
}

export class ImportAdminOverridesMetadata extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportAdminOverridesMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: ImportAdminOverridesMetadata): ImportAdminOverridesMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportAdminOverridesMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportAdminOverridesMetadata;
  static deserializeBinaryFromReader(message: ImportAdminOverridesMetadata, reader: jspb.BinaryReader): ImportAdminOverridesMetadata;
}

export namespace ImportAdminOverridesMetadata {
  export type AsObject = {
  }
}

export class CreateConsumerOverrideRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  hasOverride(): boolean;
  clearOverride(): void;
  getOverride(): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride | undefined;
  setOverride(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateConsumerOverrideRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CreateConsumerOverrideRequest): CreateConsumerOverrideRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateConsumerOverrideRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateConsumerOverrideRequest;
  static deserializeBinaryFromReader(message: CreateConsumerOverrideRequest, reader: jspb.BinaryReader): CreateConsumerOverrideRequest;
}

export namespace CreateConsumerOverrideRequest {
  export type AsObject = {
    parent: string,
    override?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject,
    force: boolean,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }
}

export class UpdateConsumerOverrideRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasOverride(): boolean;
  clearOverride(): void;
  getOverride(): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride | undefined;
  setOverride(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  hasUpdateMask(): boolean;
  clearUpdateMask(): void;
  getUpdateMask(): google_protobuf_field_mask_pb.FieldMask | undefined;
  setUpdateMask(value?: google_protobuf_field_mask_pb.FieldMask): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateConsumerOverrideRequest.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateConsumerOverrideRequest): UpdateConsumerOverrideRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UpdateConsumerOverrideRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateConsumerOverrideRequest;
  static deserializeBinaryFromReader(message: UpdateConsumerOverrideRequest, reader: jspb.BinaryReader): UpdateConsumerOverrideRequest;
}

export namespace UpdateConsumerOverrideRequest {
  export type AsObject = {
    name: string,
    override?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject,
    force: boolean,
    updateMask?: google_protobuf_field_mask_pb.FieldMask.AsObject,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }
}

export class DeleteConsumerOverrideRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeleteConsumerOverrideRequest.AsObject;
  static toObject(includeInstance: boolean, msg: DeleteConsumerOverrideRequest): DeleteConsumerOverrideRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DeleteConsumerOverrideRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeleteConsumerOverrideRequest;
  static deserializeBinaryFromReader(message: DeleteConsumerOverrideRequest, reader: jspb.BinaryReader): DeleteConsumerOverrideRequest;
}

export namespace DeleteConsumerOverrideRequest {
  export type AsObject = {
    name: string,
    force: boolean,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }
}

export class ListConsumerOverridesRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  getPageSize(): number;
  setPageSize(value: number): void;

  getPageToken(): string;
  setPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListConsumerOverridesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ListConsumerOverridesRequest): ListConsumerOverridesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListConsumerOverridesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListConsumerOverridesRequest;
  static deserializeBinaryFromReader(message: ListConsumerOverridesRequest, reader: jspb.BinaryReader): ListConsumerOverridesRequest;
}

export namespace ListConsumerOverridesRequest {
  export type AsObject = {
    parent: string,
    pageSize: number,
    pageToken: string,
  }
}

export class ListConsumerOverridesResponse extends jspb.Message {
  clearOverridesList(): void;
  getOverridesList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>;
  setOverridesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>): void;
  addOverrides(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;

  getNextPageToken(): string;
  setNextPageToken(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ListConsumerOverridesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ListConsumerOverridesResponse): ListConsumerOverridesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ListConsumerOverridesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ListConsumerOverridesResponse;
  static deserializeBinaryFromReader(message: ListConsumerOverridesResponse, reader: jspb.BinaryReader): ListConsumerOverridesResponse;
}

export namespace ListConsumerOverridesResponse {
  export type AsObject = {
    overridesList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject>,
    nextPageToken: string,
  }
}

export class BatchCreateConsumerOverridesResponse extends jspb.Message {
  clearOverridesList(): void;
  getOverridesList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>;
  setOverridesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>): void;
  addOverrides(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BatchCreateConsumerOverridesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BatchCreateConsumerOverridesResponse): BatchCreateConsumerOverridesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BatchCreateConsumerOverridesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BatchCreateConsumerOverridesResponse;
  static deserializeBinaryFromReader(message: BatchCreateConsumerOverridesResponse, reader: jspb.BinaryReader): BatchCreateConsumerOverridesResponse;
}

export namespace BatchCreateConsumerOverridesResponse {
  export type AsObject = {
    overridesList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject>,
  }
}

export class ImportConsumerOverridesRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  hasInlineSource(): boolean;
  clearInlineSource(): void;
  getInlineSource(): google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource | undefined;
  setInlineSource(value?: google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource): void;

  getForce(): boolean;
  setForce(value: boolean): void;

  clearForceOnlyList(): void;
  getForceOnlyList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>;
  setForceOnlyList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>): void;
  addForceOnly(value: google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap], index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap];

  getSourceCase(): ImportConsumerOverridesRequest.SourceCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportConsumerOverridesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ImportConsumerOverridesRequest): ImportConsumerOverridesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportConsumerOverridesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportConsumerOverridesRequest;
  static deserializeBinaryFromReader(message: ImportConsumerOverridesRequest, reader: jspb.BinaryReader): ImportConsumerOverridesRequest;
}

export namespace ImportConsumerOverridesRequest {
  export type AsObject = {
    parent: string,
    inlineSource?: google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.AsObject,
    force: boolean,
    forceOnlyList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap[keyof google_api_serviceusage_v1beta1_resources_pb.QuotaSafetyCheckMap]>,
  }

  export enum SourceCase {
    SOURCE_NOT_SET = 0,
    INLINE_SOURCE = 2,
  }
}

export class ImportConsumerOverridesResponse extends jspb.Message {
  clearOverridesList(): void;
  getOverridesList(): Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>;
  setOverridesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride>): void;
  addOverrides(value?: google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, index?: number): google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportConsumerOverridesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ImportConsumerOverridesResponse): ImportConsumerOverridesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportConsumerOverridesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportConsumerOverridesResponse;
  static deserializeBinaryFromReader(message: ImportConsumerOverridesResponse, reader: jspb.BinaryReader): ImportConsumerOverridesResponse;
}

export namespace ImportConsumerOverridesResponse {
  export type AsObject = {
    overridesList: Array<google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.AsObject>,
  }
}

export class ImportConsumerOverridesMetadata extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportConsumerOverridesMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: ImportConsumerOverridesMetadata): ImportConsumerOverridesMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportConsumerOverridesMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportConsumerOverridesMetadata;
  static deserializeBinaryFromReader(message: ImportConsumerOverridesMetadata, reader: jspb.BinaryReader): ImportConsumerOverridesMetadata;
}

export namespace ImportConsumerOverridesMetadata {
  export type AsObject = {
  }
}

export class ImportAdminQuotaPoliciesResponse extends jspb.Message {
  clearPoliciesList(): void;
  getPoliciesList(): Array<google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy>;
  setPoliciesList(value: Array<google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy>): void;
  addPolicies(value?: google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy, index?: number): google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportAdminQuotaPoliciesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ImportAdminQuotaPoliciesResponse): ImportAdminQuotaPoliciesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportAdminQuotaPoliciesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportAdminQuotaPoliciesResponse;
  static deserializeBinaryFromReader(message: ImportAdminQuotaPoliciesResponse, reader: jspb.BinaryReader): ImportAdminQuotaPoliciesResponse;
}

export namespace ImportAdminQuotaPoliciesResponse {
  export type AsObject = {
    policiesList: Array<google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy.AsObject>,
  }
}

export class ImportAdminQuotaPoliciesMetadata extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ImportAdminQuotaPoliciesMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: ImportAdminQuotaPoliciesMetadata): ImportAdminQuotaPoliciesMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ImportAdminQuotaPoliciesMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ImportAdminQuotaPoliciesMetadata;
  static deserializeBinaryFromReader(message: ImportAdminQuotaPoliciesMetadata, reader: jspb.BinaryReader): ImportAdminQuotaPoliciesMetadata;
}

export namespace ImportAdminQuotaPoliciesMetadata {
  export type AsObject = {
  }
}

export class CreateAdminQuotaPolicyMetadata extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CreateAdminQuotaPolicyMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: CreateAdminQuotaPolicyMetadata): CreateAdminQuotaPolicyMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CreateAdminQuotaPolicyMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CreateAdminQuotaPolicyMetadata;
  static deserializeBinaryFromReader(message: CreateAdminQuotaPolicyMetadata, reader: jspb.BinaryReader): CreateAdminQuotaPolicyMetadata;
}

export namespace CreateAdminQuotaPolicyMetadata {
  export type AsObject = {
  }
}

export class UpdateAdminQuotaPolicyMetadata extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UpdateAdminQuotaPolicyMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: UpdateAdminQuotaPolicyMetadata): UpdateAdminQuotaPolicyMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UpdateAdminQuotaPolicyMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UpdateAdminQuotaPolicyMetadata;
  static deserializeBinaryFromReader(message: UpdateAdminQuotaPolicyMetadata, reader: jspb.BinaryReader): UpdateAdminQuotaPolicyMetadata;
}

export namespace UpdateAdminQuotaPolicyMetadata {
  export type AsObject = {
  }
}

export class DeleteAdminQuotaPolicyMetadata extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DeleteAdminQuotaPolicyMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: DeleteAdminQuotaPolicyMetadata): DeleteAdminQuotaPolicyMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DeleteAdminQuotaPolicyMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DeleteAdminQuotaPolicyMetadata;
  static deserializeBinaryFromReader(message: DeleteAdminQuotaPolicyMetadata, reader: jspb.BinaryReader): DeleteAdminQuotaPolicyMetadata;
}

export namespace DeleteAdminQuotaPolicyMetadata {
  export type AsObject = {
  }
}

export class GenerateServiceIdentityRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GenerateServiceIdentityRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GenerateServiceIdentityRequest): GenerateServiceIdentityRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GenerateServiceIdentityRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GenerateServiceIdentityRequest;
  static deserializeBinaryFromReader(message: GenerateServiceIdentityRequest, reader: jspb.BinaryReader): GenerateServiceIdentityRequest;
}

export namespace GenerateServiceIdentityRequest {
  export type AsObject = {
    parent: string,
  }
}

export class GetServiceIdentityResponse extends jspb.Message {
  hasIdentity(): boolean;
  clearIdentity(): void;
  getIdentity(): google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity | undefined;
  setIdentity(value?: google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity): void;

  getState(): GetServiceIdentityResponse.IdentityStateMap[keyof GetServiceIdentityResponse.IdentityStateMap];
  setState(value: GetServiceIdentityResponse.IdentityStateMap[keyof GetServiceIdentityResponse.IdentityStateMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetServiceIdentityResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GetServiceIdentityResponse): GetServiceIdentityResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetServiceIdentityResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetServiceIdentityResponse;
  static deserializeBinaryFromReader(message: GetServiceIdentityResponse, reader: jspb.BinaryReader): GetServiceIdentityResponse;
}

export namespace GetServiceIdentityResponse {
  export type AsObject = {
    identity?: google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity.AsObject,
    state: GetServiceIdentityResponse.IdentityStateMap[keyof GetServiceIdentityResponse.IdentityStateMap],
  }

  export interface IdentityStateMap {
    IDENTITY_STATE_UNSPECIFIED: 0;
    ACTIVE: 1;
  }

  export const IdentityState: IdentityStateMap;
}

export class GetServiceIdentityMetadata extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GetServiceIdentityMetadata.AsObject;
  static toObject(includeInstance: boolean, msg: GetServiceIdentityMetadata): GetServiceIdentityMetadata.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GetServiceIdentityMetadata, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GetServiceIdentityMetadata;
  static deserializeBinaryFromReader(message: GetServiceIdentityMetadata, reader: jspb.BinaryReader): GetServiceIdentityMetadata;
}

export namespace GetServiceIdentityMetadata {
  export type AsObject = {
  }
}

