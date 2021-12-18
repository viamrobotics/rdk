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

export class ServoServiceAngularOffsetRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoServiceAngularOffsetRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ServoServiceAngularOffsetRequest): ServoServiceAngularOffsetRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoServiceAngularOffsetRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoServiceAngularOffsetRequest;
  static deserializeBinaryFromReader(message: ServoServiceAngularOffsetRequest, reader: jspb.BinaryReader): ServoServiceAngularOffsetRequest;
}

export namespace ServoServiceAngularOffsetRequest {
  export type AsObject = {
    name: string,
  }
}

export class ServoServiceAngularOffsetResponse extends jspb.Message {
  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ServoServiceAngularOffsetResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ServoServiceAngularOffsetResponse): ServoServiceAngularOffsetResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ServoServiceAngularOffsetResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ServoServiceAngularOffsetResponse;
  static deserializeBinaryFromReader(message: ServoServiceAngularOffsetResponse, reader: jspb.BinaryReader): ServoServiceAngularOffsetResponse;
}

export namespace ServoServiceAngularOffsetResponse {
  export type AsObject = {
    angleDeg: number,
  }
}

