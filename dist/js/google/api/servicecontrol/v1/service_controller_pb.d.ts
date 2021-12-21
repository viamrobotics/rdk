// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/service_controller.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_client_pb from "../../../../google/api/client_pb";
import * as google_api_servicecontrol_v1_check_error_pb from "../../../../google/api/servicecontrol/v1/check_error_pb";
import * as google_api_servicecontrol_v1_operation_pb from "../../../../google/api/servicecontrol/v1/operation_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as google_rpc_status_pb from "../../../../google/rpc/status_pb";

export class CheckRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  hasOperation(): boolean;
  clearOperation(): void;
  getOperation(): google_api_servicecontrol_v1_operation_pb.Operation | undefined;
  setOperation(value?: google_api_servicecontrol_v1_operation_pb.Operation): void;

  getServiceConfigId(): string;
  setServiceConfigId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CheckRequest): CheckRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckRequest;
  static deserializeBinaryFromReader(message: CheckRequest, reader: jspb.BinaryReader): CheckRequest;
}

export namespace CheckRequest {
  export type AsObject = {
    serviceName: string,
    operation?: google_api_servicecontrol_v1_operation_pb.Operation.AsObject,
    serviceConfigId: string,
  }
}

export class CheckResponse extends jspb.Message {
  getOperationId(): string;
  setOperationId(value: string): void;

  clearCheckErrorsList(): void;
  getCheckErrorsList(): Array<google_api_servicecontrol_v1_check_error_pb.CheckError>;
  setCheckErrorsList(value: Array<google_api_servicecontrol_v1_check_error_pb.CheckError>): void;
  addCheckErrors(value?: google_api_servicecontrol_v1_check_error_pb.CheckError, index?: number): google_api_servicecontrol_v1_check_error_pb.CheckError;

  getServiceConfigId(): string;
  setServiceConfigId(value: string): void;

  getServiceRolloutId(): string;
  setServiceRolloutId(value: string): void;

  hasCheckInfo(): boolean;
  clearCheckInfo(): void;
  getCheckInfo(): CheckResponse.CheckInfo | undefined;
  setCheckInfo(value?: CheckResponse.CheckInfo): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CheckResponse): CheckResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckResponse;
  static deserializeBinaryFromReader(message: CheckResponse, reader: jspb.BinaryReader): CheckResponse;
}

export namespace CheckResponse {
  export type AsObject = {
    operationId: string,
    checkErrorsList: Array<google_api_servicecontrol_v1_check_error_pb.CheckError.AsObject>,
    serviceConfigId: string,
    serviceRolloutId: string,
    checkInfo?: CheckResponse.CheckInfo.AsObject,
  }

  export class CheckInfo extends jspb.Message {
    clearUnusedArgumentsList(): void;
    getUnusedArgumentsList(): Array<string>;
    setUnusedArgumentsList(value: Array<string>): void;
    addUnusedArguments(value: string, index?: number): string;

    hasConsumerInfo(): boolean;
    clearConsumerInfo(): void;
    getConsumerInfo(): CheckResponse.ConsumerInfo | undefined;
    setConsumerInfo(value?: CheckResponse.ConsumerInfo): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CheckInfo.AsObject;
    static toObject(includeInstance: boolean, msg: CheckInfo): CheckInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CheckInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CheckInfo;
    static deserializeBinaryFromReader(message: CheckInfo, reader: jspb.BinaryReader): CheckInfo;
  }

  export namespace CheckInfo {
    export type AsObject = {
      unusedArgumentsList: Array<string>,
      consumerInfo?: CheckResponse.ConsumerInfo.AsObject,
    }
  }

  export class ConsumerInfo extends jspb.Message {
    getProjectNumber(): number;
    setProjectNumber(value: number): void;

    getType(): CheckResponse.ConsumerInfo.ConsumerTypeMap[keyof CheckResponse.ConsumerInfo.ConsumerTypeMap];
    setType(value: CheckResponse.ConsumerInfo.ConsumerTypeMap[keyof CheckResponse.ConsumerInfo.ConsumerTypeMap]): void;

    getConsumerNumber(): number;
    setConsumerNumber(value: number): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ConsumerInfo.AsObject;
    static toObject(includeInstance: boolean, msg: ConsumerInfo): ConsumerInfo.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ConsumerInfo, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ConsumerInfo;
    static deserializeBinaryFromReader(message: ConsumerInfo, reader: jspb.BinaryReader): ConsumerInfo;
  }

  export namespace ConsumerInfo {
    export type AsObject = {
      projectNumber: number,
      type: CheckResponse.ConsumerInfo.ConsumerTypeMap[keyof CheckResponse.ConsumerInfo.ConsumerTypeMap],
      consumerNumber: number,
    }

    export interface ConsumerTypeMap {
      CONSUMER_TYPE_UNSPECIFIED: 0;
      PROJECT: 1;
      FOLDER: 2;
      ORGANIZATION: 3;
      SERVICE_SPECIFIC: 4;
    }

    export const ConsumerType: ConsumerTypeMap;
  }
}

export class ReportRequest extends jspb.Message {
  getServiceName(): string;
  setServiceName(value: string): void;

  clearOperationsList(): void;
  getOperationsList(): Array<google_api_servicecontrol_v1_operation_pb.Operation>;
  setOperationsList(value: Array<google_api_servicecontrol_v1_operation_pb.Operation>): void;
  addOperations(value?: google_api_servicecontrol_v1_operation_pb.Operation, index?: number): google_api_servicecontrol_v1_operation_pb.Operation;

  getServiceConfigId(): string;
  setServiceConfigId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ReportRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ReportRequest): ReportRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ReportRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ReportRequest;
  static deserializeBinaryFromReader(message: ReportRequest, reader: jspb.BinaryReader): ReportRequest;
}

export namespace ReportRequest {
  export type AsObject = {
    serviceName: string,
    operationsList: Array<google_api_servicecontrol_v1_operation_pb.Operation.AsObject>,
    serviceConfigId: string,
  }
}

export class ReportResponse extends jspb.Message {
  clearReportErrorsList(): void;
  getReportErrorsList(): Array<ReportResponse.ReportError>;
  setReportErrorsList(value: Array<ReportResponse.ReportError>): void;
  addReportErrors(value?: ReportResponse.ReportError, index?: number): ReportResponse.ReportError;

  getServiceConfigId(): string;
  setServiceConfigId(value: string): void;

  getServiceRolloutId(): string;
  setServiceRolloutId(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ReportResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ReportResponse): ReportResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ReportResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ReportResponse;
  static deserializeBinaryFromReader(message: ReportResponse, reader: jspb.BinaryReader): ReportResponse;
}

export namespace ReportResponse {
  export type AsObject = {
    reportErrorsList: Array<ReportResponse.ReportError.AsObject>,
    serviceConfigId: string,
    serviceRolloutId: string,
  }

  export class ReportError extends jspb.Message {
    getOperationId(): string;
    setOperationId(value: string): void;

    hasStatus(): boolean;
    clearStatus(): void;
    getStatus(): google_rpc_status_pb.Status | undefined;
    setStatus(value?: google_rpc_status_pb.Status): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ReportError.AsObject;
    static toObject(includeInstance: boolean, msg: ReportError): ReportError.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ReportError, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ReportError;
    static deserializeBinaryFromReader(message: ReportError, reader: jspb.BinaryReader): ReportError;
  }

  export namespace ReportError {
    export type AsObject = {
      operationId: string,
      status?: google_rpc_status_pb.Status.AsObject,
    }
  }
}

