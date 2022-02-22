// package: proto.api.component.v1
// file: proto/api/component/v1/servo.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class ServoServiceMoveRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoServiceMoveRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ServoServiceMoveRequest): ServoServiceMoveRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoServiceMoveRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoServiceMoveRequest;
  static deserializeBinaryFromReader(message: ServoServiceMoveRequest, reader: jspb.BinaryReader): ServoServiceMoveRequest;
}

export namespace ServoServiceMoveRequest {
  export type AsObject = {
    name: string,
    angleDeg: number,
  }
}

export class ServoServiceMoveResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoServiceMoveResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ServoServiceMoveResponse): ServoServiceMoveResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoServiceMoveResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoServiceMoveResponse;
  static deserializeBinaryFromReader(message: ServoServiceMoveResponse, reader: jspb.BinaryReader): ServoServiceMoveResponse;
}

export namespace ServoServiceMoveResponse {
  export type AsObject = {
  }
}

export class ServoServiceGetPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoServiceGetPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ServoServiceGetPositionRequest): ServoServiceGetPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoServiceGetPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoServiceGetPositionRequest;
  static deserializeBinaryFromReader(message: ServoServiceGetPositionRequest, reader: jspb.BinaryReader): ServoServiceGetPositionRequest;
}

export namespace ServoServiceGetPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class ServoServiceGetPositionResponse extends jspb.Message {
  getPositionDeg(): number;
  setPositionDeg(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoServiceGetPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ServoServiceGetPositionResponse): ServoServiceGetPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoServiceGetPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoServiceGetPositionResponse;
  static deserializeBinaryFromReader(message: ServoServiceGetPositionResponse, reader: jspb.BinaryReader): ServoServiceGetPositionResponse;
}

export namespace ServoServiceGetPositionResponse {
  export type AsObject = {
    positionDeg: number,
  }
}

