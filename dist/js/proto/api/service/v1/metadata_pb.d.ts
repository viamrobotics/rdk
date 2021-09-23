// package: proto.api.service.v1
// file: proto/api/service/v1/metadata.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_httpbody_pb from "../../../../google/api/httpbody_pb";

export class ResourcesRequest extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ResourcesRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ResourcesRequest): ResourcesRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ResourcesRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ResourcesRequest;
  static deserializeBinaryFromReader(message: ResourcesRequest, reader: jspb.BinaryReader): ResourcesRequest;
}

export namespace ResourcesRequest {
  export type AsObject = {
  }
}

export class ResourceName extends jspb.Message {
  getUuid(): string;
  setUuid(value: string): void;

  getNamespace(): string;
  setNamespace(value: string): void;

  getType(): string;
  setType(value: string): void;

  getSubtype(): string;
  setSubtype(value: string): void;

  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ResourceName.AsObject;
  static toObject(includeInstance: boolean, msg: ResourceName): ResourceName.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ResourceName, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ResourceName;
  static deserializeBinaryFromReader(message: ResourceName, reader: jspb.BinaryReader): ResourceName;
}

export namespace ResourceName {
  export type AsObject = {
    uuid: string,
    namespace: string,
    type: string,
    subtype: string,
    name: string,
  }
}

export class ResourcesResponse extends jspb.Message {
  clearResourcesList(): void;
  getResourcesList(): Array<ResourceName>;
  setResourcesList(value: Array<ResourceName>): void;
  addResources(value?: ResourceName, index?: number): ResourceName;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ResourcesResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ResourcesResponse): ResourcesResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ResourcesResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ResourcesResponse;
  static deserializeBinaryFromReader(message: ResourcesResponse, reader: jspb.BinaryReader): ResourcesResponse;
}

export namespace ResourcesResponse {
  export type AsObject = {
    resourcesList: Array<ResourceName.AsObject>,
  }
}

