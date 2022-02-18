// package: proto.api.component.v1
// file: proto/api/component/v1/gantry.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class GantryServiceGetPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceGetPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceGetPositionRequest): GantryServiceGetPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceGetPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceGetPositionRequest;
  static deserializeBinaryFromReader(message: GantryServiceGetPositionRequest, reader: jspb.BinaryReader): GantryServiceGetPositionRequest;
}

export namespace GantryServiceGetPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class GantryServiceGetPositionResponse extends jspb.Message {
  clearPositionsMmList(): void;
  getPositionsMmList(): Array<number>;
  setPositionsMmList(value: Array<number>): void;
  addPositionsMm(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceGetPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceGetPositionResponse): GantryServiceGetPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceGetPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceGetPositionResponse;
  static deserializeBinaryFromReader(message: GantryServiceGetPositionResponse, reader: jspb.BinaryReader): GantryServiceGetPositionResponse;
}

export namespace GantryServiceGetPositionResponse {
  export type AsObject = {
    positionsMmList: Array<number>,
  }
}

export class GantryServiceMoveToPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  clearPositionsMmList(): void;
  getPositionsMmList(): Array<number>;
  setPositionsMmList(value: Array<number>): void;
  addPositionsMm(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceMoveToPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceMoveToPositionRequest): GantryServiceMoveToPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceMoveToPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceMoveToPositionRequest;
  static deserializeBinaryFromReader(message: GantryServiceMoveToPositionRequest, reader: jspb.BinaryReader): GantryServiceMoveToPositionRequest;
}

export namespace GantryServiceMoveToPositionRequest {
  export type AsObject = {
    name: string,
    positionsMmList: Array<number>,
  }
}

export class GantryServiceMoveToPositionResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceMoveToPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceMoveToPositionResponse): GantryServiceMoveToPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceMoveToPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceMoveToPositionResponse;
  static deserializeBinaryFromReader(message: GantryServiceMoveToPositionResponse, reader: jspb.BinaryReader): GantryServiceMoveToPositionResponse;
}

export namespace GantryServiceMoveToPositionResponse {
  export type AsObject = {
  }
}

export class GantryServiceGetLengthsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceGetLengthsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceGetLengthsRequest): GantryServiceGetLengthsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceGetLengthsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceGetLengthsRequest;
  static deserializeBinaryFromReader(message: GantryServiceGetLengthsRequest, reader: jspb.BinaryReader): GantryServiceGetLengthsRequest;
}

export namespace GantryServiceGetLengthsRequest {
  export type AsObject = {
    name: string,
  }
}

export class GantryServiceGetLengthsResponse extends jspb.Message {
  clearLengthsMmList(): void;
  getLengthsMmList(): Array<number>;
  setLengthsMmList(value: Array<number>): void;
  addLengthsMm(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceGetLengthsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceGetLengthsResponse): GantryServiceGetLengthsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceGetLengthsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceGetLengthsResponse;
  static deserializeBinaryFromReader(message: GantryServiceGetLengthsResponse, reader: jspb.BinaryReader): GantryServiceGetLengthsResponse;
}

export namespace GantryServiceGetLengthsResponse {
  export type AsObject = {
    lengthsMmList: Array<number>,
  }
}

