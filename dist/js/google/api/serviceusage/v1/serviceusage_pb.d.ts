// package: google.api.serviceusage.v1
// file: google/api/serviceusage/v1/serviceusage.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_serviceusage_v1_resources_pb from "../../../../google/api/serviceusage/v1/resources_pb";
import * as google_longrunning_operations_pb from "../../../../google/longrunning/operations_pb";
import * as google_api_client_pb from "../../../../google/api/client_pb";

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

export class EnableServiceResponse extends jspb.Message {
  hasService(): boolean;
  clearService(): void;
  getService(): google_api_serviceusage_v1_resources_pb.Service | undefined;
  setService(value?: google_api_serviceusage_v1_resources_pb.Service): void;

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
    service?: google_api_serviceusage_v1_resources_pb.Service.AsObject,
  }
}

export class DisableServiceRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getDisableDependentServices(): boolean;
  setDisableDependentServices(value: boolean): void;

  getCheckIfServiceHasUsage(): DisableServiceRequest.CheckIfServiceHasUsageMap[keyof DisableServiceRequest.CheckIfServiceHasUsageMap];
  setCheckIfServiceHasUsage(value: DisableServiceRequest.CheckIfServiceHasUsageMap[keyof DisableServiceRequest.CheckIfServiceHasUsageMap]): void;

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
    disableDependentServices: boolean,
    checkIfServiceHasUsage: DisableServiceRequest.CheckIfServiceHasUsageMap[keyof DisableServiceRequest.CheckIfServiceHasUsageMap],
  }

  export interface CheckIfServiceHasUsageMap {
    CHECK_IF_SERVICE_HAS_USAGE_UNSPECIFIED: 0;
    SKIP: 1;
    CHECK: 2;
  }

  export const CheckIfServiceHasUsage: CheckIfServiceHasUsageMap;
}

export class DisableServiceResponse extends jspb.Message {
  hasService(): boolean;
  clearService(): void;
  getService(): google_api_serviceusage_v1_resources_pb.Service | undefined;
  setService(value?: google_api_serviceusage_v1_resources_pb.Service): void;

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
    service?: google_api_serviceusage_v1_resources_pb.Service.AsObject,
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
  getServicesList(): Array<google_api_serviceusage_v1_resources_pb.Service>;
  setServicesList(value: Array<google_api_serviceusage_v1_resources_pb.Service>): void;
  addServices(value?: google_api_serviceusage_v1_resources_pb.Service, index?: number): google_api_serviceusage_v1_resources_pb.Service;

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
    servicesList: Array<google_api_serviceusage_v1_resources_pb.Service.AsObject>,
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

export class BatchEnableServicesResponse extends jspb.Message {
  clearServicesList(): void;
  getServicesList(): Array<google_api_serviceusage_v1_resources_pb.Service>;
  setServicesList(value: Array<google_api_serviceusage_v1_resources_pb.Service>): void;
  addServices(value?: google_api_serviceusage_v1_resources_pb.Service, index?: number): google_api_serviceusage_v1_resources_pb.Service;

  clearFailuresList(): void;
  getFailuresList(): Array<BatchEnableServicesResponse.EnableFailure>;
  setFailuresList(value: Array<BatchEnableServicesResponse.EnableFailure>): void;
  addFailures(value?: BatchEnableServicesResponse.EnableFailure, index?: number): BatchEnableServicesResponse.EnableFailure;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BatchEnableServicesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BatchEnableServicesResponse): BatchEnableServicesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BatchEnableServicesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BatchEnableServicesResponse;
  static deserializeBinaryFromReader(message: BatchEnableServicesResponse, reader: jspb.BinaryReader): BatchEnableServicesResponse;
}

export namespace BatchEnableServicesResponse {
  export type AsObject = {
    servicesList: Array<google_api_serviceusage_v1_resources_pb.Service.AsObject>,
    failuresList: Array<BatchEnableServicesResponse.EnableFailure.AsObject>,
  }

  export class EnableFailure extends jspb.Message {
    getServiceId(): string;
    setServiceId(value: string): void;

    getErrorMessage(): string;
    setErrorMessage(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): EnableFailure.AsObject;
    static toObject(includeInstance: boolean, msg: EnableFailure): EnableFailure.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: EnableFailure, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): EnableFailure;
    static deserializeBinaryFromReader(message: EnableFailure, reader: jspb.BinaryReader): EnableFailure;
  }

  export namespace EnableFailure {
    export type AsObject = {
      serviceId: string,
      errorMessage: string,
    }
  }
}

export class BatchGetServicesRequest extends jspb.Message {
  getParent(): string;
  setParent(value: string): void;

  clearNamesList(): void;
  getNamesList(): Array<string>;
  setNamesList(value: Array<string>): void;
  addNames(value: string, index?: number): string;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BatchGetServicesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BatchGetServicesRequest): BatchGetServicesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BatchGetServicesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BatchGetServicesRequest;
  static deserializeBinaryFromReader(message: BatchGetServicesRequest, reader: jspb.BinaryReader): BatchGetServicesRequest;
}

export namespace BatchGetServicesRequest {
  export type AsObject = {
    parent: string,
    namesList: Array<string>,
  }
}

export class BatchGetServicesResponse extends jspb.Message {
  clearServicesList(): void;
  getServicesList(): Array<google_api_serviceusage_v1_resources_pb.Service>;
  setServicesList(value: Array<google_api_serviceusage_v1_resources_pb.Service>): void;
  addServices(value?: google_api_serviceusage_v1_resources_pb.Service, index?: number): google_api_serviceusage_v1_resources_pb.Service;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BatchGetServicesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BatchGetServicesResponse): BatchGetServicesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BatchGetServicesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BatchGetServicesResponse;
  static deserializeBinaryFromReader(message: BatchGetServicesResponse, reader: jspb.BinaryReader): BatchGetServicesResponse;
}

export namespace BatchGetServicesResponse {
  export type AsObject = {
    servicesList: Array<google_api_serviceusage_v1_resources_pb.Service.AsObject>,
  }
}

