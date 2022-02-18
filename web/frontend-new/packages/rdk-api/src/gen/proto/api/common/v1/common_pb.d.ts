// package: proto.api.common.v1
// file: proto/api/common/v1/common.proto

import * as jspb from "google-protobuf";

export class BoardStatus extends jspb.Message {
  getAnalogsMap(): jspb.Map<string, AnalogStatus>;
  clearAnalogsMap(): void;
  getDigitalInterruptsMap(): jspb.Map<string, DigitalInterruptStatus>;
  clearDigitalInterruptsMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardStatus.AsObject;
  static toObject(includeInstance: boolean, msg: BoardStatus): BoardStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardStatus;
  static deserializeBinaryFromReader(message: BoardStatus, reader: jspb.BinaryReader): BoardStatus;
}

export namespace BoardStatus {
  export type AsObject = {
    analogsMap: Array<[string, AnalogStatus.AsObject]>,
    digitalInterruptsMap: Array<[string, DigitalInterruptStatus.AsObject]>,
  }
}

export class AnalogStatus extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): AnalogStatus.AsObject;
  static toObject(includeInstance: boolean, msg: AnalogStatus): AnalogStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: AnalogStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): AnalogStatus;
  static deserializeBinaryFromReader(message: AnalogStatus, reader: jspb.BinaryReader): AnalogStatus;
}

export namespace AnalogStatus {
  export type AsObject = {
    value: number,
  }
}

export class DigitalInterruptStatus extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DigitalInterruptStatus.AsObject;
  static toObject(includeInstance: boolean, msg: DigitalInterruptStatus): DigitalInterruptStatus.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DigitalInterruptStatus, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DigitalInterruptStatus;
  static deserializeBinaryFromReader(message: DigitalInterruptStatus, reader: jspb.BinaryReader): DigitalInterruptStatus;
}

export namespace DigitalInterruptStatus {
  export type AsObject = {
    value: number,
  }
}

export class Pose extends jspb.Message {
  getX(): number;
  setX(value: number): void;

  getY(): number;
  setY(value: number): void;

  getZ(): number;
  setZ(value: number): void;

  getOX(): number;
  setOX(value: number): void;

  getOY(): number;
  setOY(value: number): void;

  getOZ(): number;
  setOZ(value: number): void;

  getTheta(): number;
  setTheta(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Pose.AsObject;
  static toObject(includeInstance: boolean, msg: Pose): Pose.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Pose, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Pose;
  static deserializeBinaryFromReader(message: Pose, reader: jspb.BinaryReader): Pose;
}

export namespace Pose {
  export type AsObject = {
    x: number,
    y: number,
    z: number,
    oX: number,
    oY: number,
    oZ: number,
    theta: number,
  }
}

export class Vector3 extends jspb.Message {
  getX(): number;
  setX(value: number): void;

  getY(): number;
  setY(value: number): void;

  getZ(): number;
  setZ(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Vector3.AsObject;
  static toObject(includeInstance: boolean, msg: Vector3): Vector3.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Vector3, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Vector3;
  static deserializeBinaryFromReader(message: Vector3, reader: jspb.BinaryReader): Vector3;
}

export namespace Vector3 {
  export type AsObject = {
    x: number,
    y: number,
    z: number,
  }
}

export class BoxGeometry extends jspb.Message {
  getWidthMm(): number;
  setWidthMm(value: number): void;

  getLengthMm(): number;
  setLengthMm(value: number): void;

  getDepthMm(): number;
  setDepthMm(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoxGeometry.AsObject;
  static toObject(includeInstance: boolean, msg: BoxGeometry): BoxGeometry.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoxGeometry, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoxGeometry;
  static deserializeBinaryFromReader(message: BoxGeometry, reader: jspb.BinaryReader): BoxGeometry;
}

export namespace BoxGeometry {
  export type AsObject = {
    widthMm: number,
    lengthMm: number,
    depthMm: number,
  }
}

export class GeoPoint extends jspb.Message {
  getLatitude(): number;
  setLatitude(value: number): void;

  getLongitude(): number;
  setLongitude(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GeoPoint.AsObject;
  static toObject(includeInstance: boolean, msg: GeoPoint): GeoPoint.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GeoPoint, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GeoPoint;
  static deserializeBinaryFromReader(message: GeoPoint, reader: jspb.BinaryReader): GeoPoint;
}

export namespace GeoPoint {
  export type AsObject = {
    latitude: number,
    longitude: number,
  }
}

