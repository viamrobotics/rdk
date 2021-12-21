// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/quota_controller.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_servicecontrol_v1_metric_value_pb from "../../../../google/api/servicecontrol/v1/metric_value_pb";
import * as google_rpc_status_pb from "../../../../google/rpc/status_pb";
import * as google_api_client_pb from "../../../../google/api/client_pb";

export class AllocateQuotaRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  hasAllocateOperation(): boolean;
  clearAllocateOperation(): void;
  getAllocateOperation(): QuotaOperation | undefined;
  setAllocateOperation(value?: QuotaOperation): void;

  getServiceConfigId(): string;
  setServiceConfigId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AllocateQuotaRequest.AsObject;
  static toObject(includeInstance: boolean, msg: AllocateQuotaRequest): AllocateQuotaRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AllocateQuotaRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AllocateQuotaRequest;
  static deserializeBinaryFromReader(message: AllocateQuotaRequest, reader: jspb.BinaryReader): AllocateQuotaRequest;
}

export namespace AllocateQuotaRequest {
  export type AsObject = {
    serviceName: string,
    allocateOperation?: QuotaOperation.AsObject,
    serviceConfigId: string,
  }
}

export class QuotaOperation extends jspb.Message {
  getOperationId(): string;
  setOperationId(value: string): void;

  getMethodName(): string;
  setMethodName(value: string): void;

  getConsumerId(): string;
  setConsumerId(value: string): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  clearQuotaMetricsList(): void;
  getQuotaMetricsList(): Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet>;
  setQuotaMetricsList(value: Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet>): void;
  addQuotaMetrics(value?: google_api_servicecontrol_v1_metric_value_pb.MetricValueSet, index?: number): google_api_servicecontrol_v1_metric_value_pb.MetricValueSet;

  getQuotaMode(): QuotaOperation.QuotaModeMap[keyof QuotaOperation.QuotaModeMap];
  setQuotaMode(value: QuotaOperation.QuotaModeMap[keyof QuotaOperation.QuotaModeMap]): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): QuotaOperation.AsObject;
  static toObject(includeInstance: boolean, msg: QuotaOperation): QuotaOperation.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: QuotaOperation, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): QuotaOperation;
  static deserializeBinaryFromReader(message: QuotaOperation, reader: jspb.BinaryReader): QuotaOperation;
}

export namespace QuotaOperation {
  export type AsObject = {
    operationId: string,
    methodName: string,
    consumerId: string,
    labelsMap: Array<[string, string]>,
    quotaMetricsList: Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet.AsObject>,
    quotaMode: QuotaOperation.QuotaModeMap[keyof QuotaOperation.QuotaModeMap],
  }

  export interface QuotaModeMap {
    UNSPECIFIED: 0;
    NORMAL: 1;
    BEST_EFFORT: 2;
    CHECK_ONLY: 3;
    QUERY_ONLY: 4;
    ADJUST_ONLY: 5;
  }

  export const QuotaMode: QuotaModeMap;
}

export class AllocateQuotaResponse extends jspb.Message {
  getOperationId(): string;
  setOperationId(value: string): void;

  clearAllocateErrorsList(): void;
  getAllocateErrorsList(): Array<QuotaError>;
  setAllocateErrorsList(value: Array<QuotaError>): void;
  addAllocateErrors(value?: QuotaError, index?: number): QuotaError;

  clearQuotaMetricsList(): void;
  getQuotaMetricsList(): Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet>;
  setQuotaMetricsList(value: Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet>): void;
  addQuotaMetrics(value?: google_api_servicecontrol_v1_metric_value_pb.MetricValueSet, index?: number): google_api_servicecontrol_v1_metric_value_pb.MetricValueSet;

  getServiceConfigId(): string;
  setServiceConfigId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AllocateQuotaResponse.AsObject;
  static toObject(includeInstance: boolean, msg: AllocateQuotaResponse): AllocateQuotaResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AllocateQuotaResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AllocateQuotaResponse;
  static deserializeBinaryFromReader(message: AllocateQuotaResponse, reader: jspb.BinaryReader): AllocateQuotaResponse;
}

export namespace AllocateQuotaResponse {
  export type AsObject = {
    operationId: string,
    allocateErrorsList: Array<QuotaError.AsObject>,
    quotaMetricsList: Array<google_api_servicecontrol_v1_metric_value_pb.MetricValueSet.AsObject>,
    serviceConfigId: string,
  }
}

export class QuotaError extends jspb.Message {
  getCode(): QuotaError.CodeMap[keyof QuotaError.CodeMap];
  setCode(value: QuotaError.CodeMap[keyof QuotaError.CodeMap]): void;

  getSubject(): string;
  setSubject(value: string): void;

  getDescription(): string;
  setDescription(value: string): void;

  hasStatus(): boolean;
  clearStatus(): void;
  getStatus(): google_rpc_status_pb.Status | undefined;
  setStatus(value?: google_rpc_status_pb.Status): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): QuotaError.AsObject;
  static toObject(includeInstance: boolean, msg: QuotaError): QuotaError.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: QuotaError, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): QuotaError;
  static deserializeBinaryFromReader(message: QuotaError, reader: jspb.BinaryReader): QuotaError;
}

export namespace QuotaError {
  export type AsObject = {
    code: QuotaError.CodeMap[keyof QuotaError.CodeMap],
    subject: string,
    description: string,
    status?: google_rpc_status_pb.Status.AsObject,
  }

  export interface CodeMap {
    UNSPECIFIED: 0;
    RESOURCE_EXHAUSTED: 8;
    BILLING_NOT_ACTIVE: 107;
    PROJECT_DELETED: 108;
    API_KEY_INVALID: 105;
    API_KEY_EXPIRED: 112;
  }

  export const Code: CodeMap;
}

