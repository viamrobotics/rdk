// package: proto.api.component.v1
// file: proto/api/component/v1/arm.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as proto_api_common_v1_common_pb from "../../../../proto/api/common/v1/common_pb";

export class ArmJointPositions extends jspb.Message {
  clearDegreesList(): void;
  getDegreesList(): Array<number>;
  setDegreesList(value: Array<number>): void;
  addDegrees(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmJointPositions.AsObject;
  static toObject(includeInstance: boolean, msg: ArmJointPositions): ArmJointPositions.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmJointPositions, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmJointPositions;
  static deserializeBinaryFromReader(message: ArmJointPositions, reader: jspb.BinaryReader): ArmJointPositions;
}

export namespace ArmJointPositions {
  export type AsObject = {
    degreesList: Array<number>,
  }
}

export class ArmServiceCurrentPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceCurrentPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceCurrentPositionRequest): ArmServiceCurrentPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceCurrentPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceCurrentPositionRequest;
  static deserializeBinaryFromReader(message: ArmServiceCurrentPositionRequest, reader: jspb.BinaryReader): ArmServiceCurrentPositionRequest;
}

export namespace ArmServiceCurrentPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmServiceCurrentPositionResponse extends jspb.Message {
  hasPosition(): boolean;
  clearPosition(): void;
  getPosition(): proto_api_common_v1_common_pb.Pose | undefined;
  setPosition(value?: proto_api_common_v1_common_pb.Pose): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceCurrentPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceCurrentPositionResponse): ArmServiceCurrentPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceCurrentPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceCurrentPositionResponse;
  static deserializeBinaryFromReader(message: ArmServiceCurrentPositionResponse, reader: jspb.BinaryReader): ArmServiceCurrentPositionResponse;
}

export namespace ArmServiceCurrentPositionResponse {
  export type AsObject = {
    position?: proto_api_common_v1_common_pb.Pose.AsObject,
  }
}

export class ArmServiceCurrentJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceCurrentJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceCurrentJointPositionsRequest): ArmServiceCurrentJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceCurrentJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceCurrentJointPositionsRequest;
  static deserializeBinaryFromReader(message: ArmServiceCurrentJointPositionsRequest, reader: jspb.BinaryReader): ArmServiceCurrentJointPositionsRequest;
}

export namespace ArmServiceCurrentJointPositionsRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmServiceCurrentJointPositionsResponse extends jspb.Message {
  hasPositions(): boolean;
  clearPositions(): void;
  getPositions(): ArmJointPositions | undefined;
  setPositions(value?: ArmJointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceCurrentJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceCurrentJointPositionsResponse): ArmServiceCurrentJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceCurrentJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceCurrentJointPositionsResponse;
  static deserializeBinaryFromReader(message: ArmServiceCurrentJointPositionsResponse, reader: jspb.BinaryReader): ArmServiceCurrentJointPositionsResponse;
}

export namespace ArmServiceCurrentJointPositionsResponse {
  export type AsObject = {
    positions?: ArmJointPositions.AsObject,
  }
}

export class ArmServiceMoveToPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): proto_api_common_v1_common_pb.Pose | undefined;
  setTo(value?: proto_api_common_v1_common_pb.Pose): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceMoveToPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceMoveToPositionRequest): ArmServiceMoveToPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceMoveToPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceMoveToPositionRequest;
  static deserializeBinaryFromReader(message: ArmServiceMoveToPositionRequest, reader: jspb.BinaryReader): ArmServiceMoveToPositionRequest;
}

export namespace ArmServiceMoveToPositionRequest {
  export type AsObject = {
    name: string,
    to?: proto_api_common_v1_common_pb.Pose.AsObject,
  }
}

export class ArmServiceMoveToPositionResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceMoveToPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceMoveToPositionResponse): ArmServiceMoveToPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceMoveToPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceMoveToPositionResponse;
  static deserializeBinaryFromReader(message: ArmServiceMoveToPositionResponse, reader: jspb.BinaryReader): ArmServiceMoveToPositionResponse;
}

export namespace ArmServiceMoveToPositionResponse {
  export type AsObject = {
  }
}

export class ArmServiceMoveToJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): ArmJointPositions | undefined;
  setTo(value?: ArmJointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceMoveToJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceMoveToJointPositionsRequest): ArmServiceMoveToJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceMoveToJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceMoveToJointPositionsRequest;
  static deserializeBinaryFromReader(message: ArmServiceMoveToJointPositionsRequest, reader: jspb.BinaryReader): ArmServiceMoveToJointPositionsRequest;
}

export namespace ArmServiceMoveToJointPositionsRequest {
  export type AsObject = {
    name: string,
    to?: ArmJointPositions.AsObject,
  }
}

export class ArmServiceMoveToJointPositionsResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceMoveToJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceMoveToJointPositionsResponse): ArmServiceMoveToJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceMoveToJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceMoveToJointPositionsResponse;
  static deserializeBinaryFromReader(message: ArmServiceMoveToJointPositionsResponse, reader: jspb.BinaryReader): ArmServiceMoveToJointPositionsResponse;
}

export namespace ArmServiceMoveToJointPositionsResponse {
  export type AsObject = {
  }
}

export class ArmServiceJointMoveDeltaRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getJoint(): number;
  setJoint(value: number): void;

  getAmountDegs(): number;
  setAmountDegs(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceJointMoveDeltaRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceJointMoveDeltaRequest): ArmServiceJointMoveDeltaRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceJointMoveDeltaRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceJointMoveDeltaRequest;
  static deserializeBinaryFromReader(message: ArmServiceJointMoveDeltaRequest, reader: jspb.BinaryReader): ArmServiceJointMoveDeltaRequest;
}

export namespace ArmServiceJointMoveDeltaRequest {
  export type AsObject = {
    name: string,
    joint: number,
    amountDegs: number,
  }
}

export class ArmServiceJointMoveDeltaResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceJointMoveDeltaResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceJointMoveDeltaResponse): ArmServiceJointMoveDeltaResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceJointMoveDeltaResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceJointMoveDeltaResponse;
  static deserializeBinaryFromReader(message: ArmServiceJointMoveDeltaResponse, reader: jspb.BinaryReader): ArmServiceJointMoveDeltaResponse;
}

export namespace ArmServiceJointMoveDeltaResponse {
  export type AsObject = {
  }
}

