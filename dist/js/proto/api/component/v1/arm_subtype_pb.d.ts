// package: proto.api.component.v1
// file: proto/api/component/v1/arm_subtype.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_httpbody_pb from "../../../../google/api/httpbody_pb";

export class Position extends jspb.Message {
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
  toObject(includeInstance?: boolean): Position.AsObject;
  static toObject(includeInstance: boolean, msg: Position): Position.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Position, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Position;
  static deserializeBinaryFromReader(message: Position, reader: jspb.BinaryReader): Position;
}

export namespace Position {
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

export class JointPositions extends jspb.Message {
  clearDegreesList(): void;
  getDegreesList(): Array<number>;
  setDegreesList(value: Array<number>): void;
  addDegrees(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JointPositions.AsObject;
  static toObject(includeInstance: boolean, msg: JointPositions): JointPositions.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JointPositions, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JointPositions;
  static deserializeBinaryFromReader(message: JointPositions, reader: jspb.BinaryReader): JointPositions;
}

export namespace JointPositions {
  export type AsObject = {
    degreesList: Array<number>,
  }
}

export class CurrentPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CurrentPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CurrentPositionRequest): CurrentPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CurrentPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CurrentPositionRequest;
  static deserializeBinaryFromReader(message: CurrentPositionRequest, reader: jspb.BinaryReader): CurrentPositionRequest;
}

export namespace CurrentPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class CurrentPositionResponse extends jspb.Message {
  hasPosition(): boolean;
  clearPosition(): void;
  getPosition(): Position | undefined;
  setPosition(value?: Position): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CurrentPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CurrentPositionResponse): CurrentPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CurrentPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CurrentPositionResponse;
  static deserializeBinaryFromReader(message: CurrentPositionResponse, reader: jspb.BinaryReader): CurrentPositionResponse;
}

export namespace CurrentPositionResponse {
  export type AsObject = {
    position?: Position.AsObject,
  }
}

export class CurrentJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CurrentJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CurrentJointPositionsRequest): CurrentJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CurrentJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CurrentJointPositionsRequest;
  static deserializeBinaryFromReader(message: CurrentJointPositionsRequest, reader: jspb.BinaryReader): CurrentJointPositionsRequest;
}

export namespace CurrentJointPositionsRequest {
  export type AsObject = {
    name: string,
  }
}

export class CurrentJointPositionsResponse extends jspb.Message {
  hasPositions(): boolean;
  clearPositions(): void;
  getPositions(): JointPositions | undefined;
  setPositions(value?: JointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CurrentJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CurrentJointPositionsResponse): CurrentJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CurrentJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CurrentJointPositionsResponse;
  static deserializeBinaryFromReader(message: CurrentJointPositionsResponse, reader: jspb.BinaryReader): CurrentJointPositionsResponse;
}

export namespace CurrentJointPositionsResponse {
  export type AsObject = {
    positions?: JointPositions.AsObject,
  }
}

export class MoveToPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): Position | undefined;
  setTo(value?: Position): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MoveToPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MoveToPositionRequest): MoveToPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MoveToPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MoveToPositionRequest;
  static deserializeBinaryFromReader(message: MoveToPositionRequest, reader: jspb.BinaryReader): MoveToPositionRequest;
}

export namespace MoveToPositionRequest {
  export type AsObject = {
    name: string,
    to?: Position.AsObject,
  }
}

export class MoveToPositionResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MoveToPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MoveToPositionResponse): MoveToPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MoveToPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MoveToPositionResponse;
  static deserializeBinaryFromReader(message: MoveToPositionResponse, reader: jspb.BinaryReader): MoveToPositionResponse;
}

export namespace MoveToPositionResponse {
  export type AsObject = {
  }
}

export class MoveToJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): JointPositions | undefined;
  setTo(value?: JointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MoveToJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MoveToJointPositionsRequest): MoveToJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MoveToJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MoveToJointPositionsRequest;
  static deserializeBinaryFromReader(message: MoveToJointPositionsRequest, reader: jspb.BinaryReader): MoveToJointPositionsRequest;
}

export namespace MoveToJointPositionsRequest {
  export type AsObject = {
    name: string,
    to?: JointPositions.AsObject,
  }
}

export class MoveToJointPositionsResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MoveToJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MoveToJointPositionsResponse): MoveToJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MoveToJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MoveToJointPositionsResponse;
  static deserializeBinaryFromReader(message: MoveToJointPositionsResponse, reader: jspb.BinaryReader): MoveToJointPositionsResponse;
}

export namespace MoveToJointPositionsResponse {
  export type AsObject = {
  }
}

export class JointMoveDeltaRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getJoint(): number;
  setJoint(value: number): void;

  getAmountDegs(): number;
  setAmountDegs(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JointMoveDeltaRequest.AsObject;
  static toObject(includeInstance: boolean, msg: JointMoveDeltaRequest): JointMoveDeltaRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JointMoveDeltaRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JointMoveDeltaRequest;
  static deserializeBinaryFromReader(message: JointMoveDeltaRequest, reader: jspb.BinaryReader): JointMoveDeltaRequest;
}

export namespace JointMoveDeltaRequest {
  export type AsObject = {
    name: string,
    joint: number,
    amountDegs: number,
  }
}

export class JointMoveDeltaResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): JointMoveDeltaResponse.AsObject;
  static toObject(includeInstance: boolean, msg: JointMoveDeltaResponse): JointMoveDeltaResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: JointMoveDeltaResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): JointMoveDeltaResponse;
  static deserializeBinaryFromReader(message: JointMoveDeltaResponse, reader: jspb.BinaryReader): JointMoveDeltaResponse;
}

export namespace JointMoveDeltaResponse {
  export type AsObject = {
  }
}

