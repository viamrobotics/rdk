// package: proto.api.component.v1
// file: proto/api/component/v1/imu.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class AngularVelocity extends jspb.Message {
  getX(): number;
  setX(value: number): void;

  getY(): number;
  setY(value: number): void;

  getZ(): number;
  setZ(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AngularVelocity.AsObject;
  static toObject(includeInstance: boolean, msg: AngularVelocity): AngularVelocity.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AngularVelocity, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AngularVelocity;
  static deserializeBinaryFromReader(message: AngularVelocity, reader: jspb.BinaryReader): AngularVelocity;
}

export namespace AngularVelocity {
  export type AsObject = {
    x: number,
    y: number,
    z: number,
  }
}

export class EulerAngles extends jspb.Message {
  getRoll(): number;
  setRoll(value: number): void;

  getPitch(): number;
  setPitch(value: number): void;

  getYaw(): number;
  setYaw(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EulerAngles.AsObject;
  static toObject(includeInstance: boolean, msg: EulerAngles): EulerAngles.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EulerAngles, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EulerAngles;
  static deserializeBinaryFromReader(message: EulerAngles, reader: jspb.BinaryReader): EulerAngles;
}

export namespace EulerAngles {
  export type AsObject = {
    roll: number,
    pitch: number,
    yaw: number,
  }
}

export class IMUServiceAngularVelocityRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceAngularVelocityRequest.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceAngularVelocityRequest): IMUServiceAngularVelocityRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceAngularVelocityRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceAngularVelocityRequest;
  static deserializeBinaryFromReader(message: IMUServiceAngularVelocityRequest, reader: jspb.BinaryReader): IMUServiceAngularVelocityRequest;
}

export namespace IMUServiceAngularVelocityRequest {
  export type AsObject = {
    name: string,
  }
}

export class IMUServiceAngularVelocityResponse extends jspb.Message {
  hasAngularVelocity(): boolean;
  clearAngularVelocity(): void;
  getAngularVelocity(): AngularVelocity | undefined;
  setAngularVelocity(value?: AngularVelocity): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceAngularVelocityResponse.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceAngularVelocityResponse): IMUServiceAngularVelocityResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceAngularVelocityResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceAngularVelocityResponse;
  static deserializeBinaryFromReader(message: IMUServiceAngularVelocityResponse, reader: jspb.BinaryReader): IMUServiceAngularVelocityResponse;
}

export namespace IMUServiceAngularVelocityResponse {
  export type AsObject = {
    angularVelocity?: AngularVelocity.AsObject,
  }
}

export class IMUServiceOrientationRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceOrientationRequest.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceOrientationRequest): IMUServiceOrientationRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceOrientationRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceOrientationRequest;
  static deserializeBinaryFromReader(message: IMUServiceOrientationRequest, reader: jspb.BinaryReader): IMUServiceOrientationRequest;
}

export namespace IMUServiceOrientationRequest {
  export type AsObject = {
    name: string,
  }
}

export class IMUServiceOrientationResponse extends jspb.Message {
  hasOrientation(): boolean;
  clearOrientation(): void;
  getOrientation(): EulerAngles | undefined;
  setOrientation(value?: EulerAngles): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceOrientationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceOrientationResponse): IMUServiceOrientationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceOrientationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceOrientationResponse;
  static deserializeBinaryFromReader(message: IMUServiceOrientationResponse, reader: jspb.BinaryReader): IMUServiceOrientationResponse;
}

export namespace IMUServiceOrientationResponse {
  export type AsObject = {
    orientation?: EulerAngles.AsObject,
  }
}

