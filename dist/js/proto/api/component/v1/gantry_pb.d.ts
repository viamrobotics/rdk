// package: proto.api.component.v1
// file: proto/api/component/v1/gantry.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class GantryServiceCurrentPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceCurrentPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceCurrentPositionRequest): GantryServiceCurrentPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceCurrentPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceCurrentPositionRequest;
  static deserializeBinaryFromReader(message: GantryServiceCurrentPositionRequest, reader: jspb.BinaryReader): GantryServiceCurrentPositionRequest;
}

export namespace GantryServiceCurrentPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class GantryServiceCurrentPositionResponse extends jspb.Message {
  clearPositionsList(): void;
  getPositionsList(): Array<number>;
  setPositionsList(value: Array<number>): void;
  addPositions(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceCurrentPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceCurrentPositionResponse): GantryServiceCurrentPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceCurrentPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceCurrentPositionResponse;
  static deserializeBinaryFromReader(message: GantryServiceCurrentPositionResponse, reader: jspb.BinaryReader): GantryServiceCurrentPositionResponse;
}

export namespace GantryServiceCurrentPositionResponse {
  export type AsObject = {
    positionsList: Array<number>,
  }
}

export class GantryServiceMoveToPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  clearPositionsList(): void;
  getPositionsList(): Array<number>;
  setPositionsList(value: Array<number>): void;
  addPositions(value: number, index?: number): number;

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
    positionsList: Array<number>,
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

export class GantryServiceLengthsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceLengthsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceLengthsRequest): GantryServiceLengthsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceLengthsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceLengthsRequest;
  static deserializeBinaryFromReader(message: GantryServiceLengthsRequest, reader: jspb.BinaryReader): GantryServiceLengthsRequest;
}

export namespace GantryServiceLengthsRequest {
  export type AsObject = {
    name: string,
  }
}

export class GantryServiceLengthsResponse extends jspb.Message {
  clearLengthsList(): void;
  getLengthsList(): Array<number>;
  setLengthsList(value: Array<number>): void;
  addLengths(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GantryServiceLengthsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GantryServiceLengthsResponse): GantryServiceLengthsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GantryServiceLengthsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GantryServiceLengthsResponse;
  static deserializeBinaryFromReader(message: GantryServiceLengthsResponse, reader: jspb.BinaryReader): GantryServiceLengthsResponse;
}

export namespace GantryServiceLengthsResponse {
  export type AsObject = {
    lengthsList: Array<number>,
  }
}

