// package: proto.api.component.v1
// file: proto/api/component/v1/gps.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as proto_api_common_v1_common_pb from "../../../../proto/api/common/v1/common_pb";

export class GPSServiceReadLocationRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GPSServiceReadLocationRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GPSServiceReadLocationRequest): GPSServiceReadLocationRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GPSServiceReadLocationRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GPSServiceReadLocationRequest;
  static deserializeBinaryFromReader(message: GPSServiceReadLocationRequest, reader: jspb.BinaryReader): GPSServiceReadLocationRequest;
}

export namespace GPSServiceReadLocationRequest {
  export type AsObject = {
    name: string,
  }
}

export class GPSServiceReadLocationResponse extends jspb.Message {
  hasCoordinate(): boolean;
  clearCoordinate(): void;
  getCoordinate(): proto_api_common_v1_common_pb.GeoPoint | undefined;
  setCoordinate(value?: proto_api_common_v1_common_pb.GeoPoint): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GPSServiceReadLocationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GPSServiceReadLocationResponse): GPSServiceReadLocationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GPSServiceReadLocationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GPSServiceReadLocationResponse;
  static deserializeBinaryFromReader(message: GPSServiceReadLocationResponse, reader: jspb.BinaryReader): GPSServiceReadLocationResponse;
}

export namespace GPSServiceReadLocationResponse {
  export type AsObject = {
    coordinate?: proto_api_common_v1_common_pb.GeoPoint.AsObject,
  }
}

export class GPSServiceReadAltitudeRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GPSServiceReadAltitudeRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GPSServiceReadAltitudeRequest): GPSServiceReadAltitudeRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GPSServiceReadAltitudeRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GPSServiceReadAltitudeRequest;
  static deserializeBinaryFromReader(message: GPSServiceReadAltitudeRequest, reader: jspb.BinaryReader): GPSServiceReadAltitudeRequest;
}

export namespace GPSServiceReadAltitudeRequest {
  export type AsObject = {
    name: string,
  }
}

export class GPSServiceReadAltitudeResponse extends jspb.Message {
  getAltitudeMeters(): number;
  setAltitudeMeters(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GPSServiceReadAltitudeResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GPSServiceReadAltitudeResponse): GPSServiceReadAltitudeResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GPSServiceReadAltitudeResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GPSServiceReadAltitudeResponse;
  static deserializeBinaryFromReader(message: GPSServiceReadAltitudeResponse, reader: jspb.BinaryReader): GPSServiceReadAltitudeResponse;
}

export namespace GPSServiceReadAltitudeResponse {
  export type AsObject = {
    altitudeMeters: number,
  }
}

export class GPSServiceReadSpeedRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GPSServiceReadSpeedRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GPSServiceReadSpeedRequest): GPSServiceReadSpeedRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GPSServiceReadSpeedRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GPSServiceReadSpeedRequest;
  static deserializeBinaryFromReader(message: GPSServiceReadSpeedRequest, reader: jspb.BinaryReader): GPSServiceReadSpeedRequest;
}

export namespace GPSServiceReadSpeedRequest {
  export type AsObject = {
    name: string,
  }
}

export class GPSServiceReadSpeedResponse extends jspb.Message {
  getSpeedMmPerSec(): number;
  setSpeedMmPerSec(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GPSServiceReadSpeedResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GPSServiceReadSpeedResponse): GPSServiceReadSpeedResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GPSServiceReadSpeedResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GPSServiceReadSpeedResponse;
  static deserializeBinaryFromReader(message: GPSServiceReadSpeedResponse, reader: jspb.BinaryReader): GPSServiceReadSpeedResponse;
}

export namespace GPSServiceReadSpeedResponse {
  export type AsObject = {
    speedMmPerSec: number,
  }
}

