// package: proto.api.component.v1
// file: proto/api/component/v1/imu.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class AngularVelocity extends jspb.Message {
  getXDegsPerSec(): number;
  setXDegsPerSec(value: number): void;

  getYDegsPerSec(): number;
  setYDegsPerSec(value: number): void;

  getZDegsPerSec(): number;
  setZDegsPerSec(value: number): void;

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
    xDegsPerSec: number,
    yDegsPerSec: number,
    zDegsPerSec: number,
  }
}

export class EulerAngles extends jspb.Message {
  getRollDeg(): number;
  setRollDeg(value: number): void;

  getPitchDeg(): number;
  setPitchDeg(value: number): void;

  getYawDeg(): number;
  setYawDeg(value: number): void;

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
    rollDeg: number,
    pitchDeg: number,
    yawDeg: number,
  }
}

export class IMUServiceReadAngularVelocityRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceReadAngularVelocityRequest.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceReadAngularVelocityRequest): IMUServiceReadAngularVelocityRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceReadAngularVelocityRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceReadAngularVelocityRequest;
  static deserializeBinaryFromReader(message: IMUServiceReadAngularVelocityRequest, reader: jspb.BinaryReader): IMUServiceReadAngularVelocityRequest;
}

export namespace IMUServiceReadAngularVelocityRequest {
  export type AsObject = {
    name: string,
  }
}

export class IMUServiceReadAngularVelocityResponse extends jspb.Message {
  hasAngularVelocity(): boolean;
  clearAngularVelocity(): void;
  getAngularVelocity(): AngularVelocity | undefined;
  setAngularVelocity(value?: AngularVelocity): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceReadAngularVelocityResponse.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceReadAngularVelocityResponse): IMUServiceReadAngularVelocityResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceReadAngularVelocityResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceReadAngularVelocityResponse;
  static deserializeBinaryFromReader(message: IMUServiceReadAngularVelocityResponse, reader: jspb.BinaryReader): IMUServiceReadAngularVelocityResponse;
}

export namespace IMUServiceReadAngularVelocityResponse {
  export type AsObject = {
    angularVelocity?: AngularVelocity.AsObject,
  }
}

export class IMUServiceReadOrientationRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceReadOrientationRequest.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceReadOrientationRequest): IMUServiceReadOrientationRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceReadOrientationRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceReadOrientationRequest;
  static deserializeBinaryFromReader(message: IMUServiceReadOrientationRequest, reader: jspb.BinaryReader): IMUServiceReadOrientationRequest;
}

export namespace IMUServiceReadOrientationRequest {
  export type AsObject = {
    name: string,
  }
}

export class IMUServiceReadOrientationResponse extends jspb.Message {
  hasOrientation(): boolean;
  clearOrientation(): void;
  getOrientation(): EulerAngles | undefined;
  setOrientation(value?: EulerAngles): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUServiceReadOrientationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: IMUServiceReadOrientationResponse): IMUServiceReadOrientationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUServiceReadOrientationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUServiceReadOrientationResponse;
  static deserializeBinaryFromReader(message: IMUServiceReadOrientationResponse, reader: jspb.BinaryReader): IMUServiceReadOrientationResponse;
}

export namespace IMUServiceReadOrientationResponse {
  export type AsObject = {
    orientation?: EulerAngles.AsObject,
  }
}

