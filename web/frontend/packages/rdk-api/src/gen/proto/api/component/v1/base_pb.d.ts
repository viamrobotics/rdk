// package: proto.api.component.v1
// file: proto/api/component/v1/base.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class BaseServiceMoveStraightRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getDistanceMm(): number;
  setDistanceMm(value: number): void;

  getMmPerSec(): number;
  setMmPerSec(value: number): void;

  getBlock(): boolean;
  setBlock(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceMoveStraightRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceMoveStraightRequest): BaseServiceMoveStraightRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceMoveStraightRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceMoveStraightRequest;
  static deserializeBinaryFromReader(message: BaseServiceMoveStraightRequest, reader: jspb.BinaryReader): BaseServiceMoveStraightRequest;
}

export namespace BaseServiceMoveStraightRequest {
  export type AsObject = {
    name: string,
    distanceMm: number,
    mmPerSec: number,
    block: boolean,
  }
}

export class BaseServiceMoveStraightResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceMoveStraightResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceMoveStraightResponse): BaseServiceMoveStraightResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceMoveStraightResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceMoveStraightResponse;
  static deserializeBinaryFromReader(message: BaseServiceMoveStraightResponse, reader: jspb.BinaryReader): BaseServiceMoveStraightResponse;
}

export namespace BaseServiceMoveStraightResponse {
  export type AsObject = {
  }
}

export class BaseServiceMoveArcRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getDistanceMm(): number;
  setDistanceMm(value: number): void;

  getMmPerSec(): number;
  setMmPerSec(value: number): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  getBlock(): boolean;
  setBlock(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceMoveArcRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceMoveArcRequest): BaseServiceMoveArcRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceMoveArcRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceMoveArcRequest;
  static deserializeBinaryFromReader(message: BaseServiceMoveArcRequest, reader: jspb.BinaryReader): BaseServiceMoveArcRequest;
}

export namespace BaseServiceMoveArcRequest {
  export type AsObject = {
    name: string,
    distanceMm: number,
    mmPerSec: number,
    angleDeg: number,
    block: boolean,
  }
}

export class BaseServiceMoveArcResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceMoveArcResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceMoveArcResponse): BaseServiceMoveArcResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceMoveArcResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceMoveArcResponse;
  static deserializeBinaryFromReader(message: BaseServiceMoveArcResponse, reader: jspb.BinaryReader): BaseServiceMoveArcResponse;
}

export namespace BaseServiceMoveArcResponse {
  export type AsObject = {
  }
}

export class BaseServiceSpinRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getAngleDeg(): number;
  setAngleDeg(value: number): void;

  getDegsPerSec(): number;
  setDegsPerSec(value: number): void;

  getBlock(): boolean;
  setBlock(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceSpinRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceSpinRequest): BaseServiceSpinRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceSpinRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceSpinRequest;
  static deserializeBinaryFromReader(message: BaseServiceSpinRequest, reader: jspb.BinaryReader): BaseServiceSpinRequest;
}

export namespace BaseServiceSpinRequest {
  export type AsObject = {
    name: string,
    angleDeg: number,
    degsPerSec: number,
    block: boolean,
  }
}

export class BaseServiceSpinResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceSpinResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceSpinResponse): BaseServiceSpinResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceSpinResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceSpinResponse;
  static deserializeBinaryFromReader(message: BaseServiceSpinResponse, reader: jspb.BinaryReader): BaseServiceSpinResponse;
}

export namespace BaseServiceSpinResponse {
  export type AsObject = {
  }
}

export class BaseServiceStopRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceStopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceStopRequest): BaseServiceStopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceStopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceStopRequest;
  static deserializeBinaryFromReader(message: BaseServiceStopRequest, reader: jspb.BinaryReader): BaseServiceStopRequest;
}

export namespace BaseServiceStopRequest {
  export type AsObject = {
    name: string,
  }
}

export class BaseServiceStopResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BaseServiceStopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BaseServiceStopResponse): BaseServiceStopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BaseServiceStopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BaseServiceStopResponse;
  static deserializeBinaryFromReader(message: BaseServiceStopResponse, reader: jspb.BinaryReader): BaseServiceStopResponse;
}

export namespace BaseServiceStopResponse {
  export type AsObject = {
  }
}

