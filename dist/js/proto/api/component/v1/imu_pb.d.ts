// package: proto.api.component.v1
// file: proto/api/component/v1/imu.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_httpbody_pb from "../../../../google/api/httpbody_pb";
import * as proto_api_common_v1_common_pb from "../../../../proto/api/common/v1/common_pb";

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

export class IMUAngularVelocityRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUAngularVelocityRequest.AsObject;
  static toObject(includeInstance: boolean, msg: IMUAngularVelocityRequest): IMUAngularVelocityRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUAngularVelocityRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUAngularVelocityRequest;
  static deserializeBinaryFromReader(message: IMUAngularVelocityRequest, reader: jspb.BinaryReader): IMUAngularVelocityRequest;
}

export namespace IMUAngularVelocityRequest {
  export type AsObject = {
    name: string,
  }
}

export class IMUAngularVelocityResponse extends jspb.Message {
  hasAngularVelocity(): boolean;
  clearAngularVelocity(): void;
  getAngularVelocity(): AngularVelocity | undefined;
  setAngularVelocity(value?: AngularVelocity): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUAngularVelocityResponse.AsObject;
  static toObject(includeInstance: boolean, msg: IMUAngularVelocityResponse): IMUAngularVelocityResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUAngularVelocityResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUAngularVelocityResponse;
  static deserializeBinaryFromReader(message: IMUAngularVelocityResponse, reader: jspb.BinaryReader): IMUAngularVelocityResponse;
}

export namespace IMUAngularVelocityResponse {
  export type AsObject = {
    angularVelocity?: AngularVelocity.AsObject,
  }
}

export class IMUOrientationRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUOrientationRequest.AsObject;
  static toObject(includeInstance: boolean, msg: IMUOrientationRequest): IMUOrientationRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUOrientationRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUOrientationRequest;
  static deserializeBinaryFromReader(message: IMUOrientationRequest, reader: jspb.BinaryReader): IMUOrientationRequest;
}

export namespace IMUOrientationRequest {
  export type AsObject = {
    name: string,
  }
}

export class IMUOrientationResponse extends jspb.Message {
  hasOrientation(): boolean;
  clearOrientation(): void;
  getOrientation(): EulerAngles | undefined;
  setOrientation(value?: EulerAngles): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IMUOrientationResponse.AsObject;
  static toObject(includeInstance: boolean, msg: IMUOrientationResponse): IMUOrientationResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IMUOrientationResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IMUOrientationResponse;
  static deserializeBinaryFromReader(message: IMUOrientationResponse, reader: jspb.BinaryReader): IMUOrientationResponse;
}

export namespace IMUOrientationResponse {
  export type AsObject = {
    orientation?: EulerAngles.AsObject,
  }
}

