// package: proto.api.component.v1
// file: proto/api/component/v1/arm_subtype.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_httpbody_pb from "../../../../google/api/httpbody_pb";
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

export class ArmSubtypeServiceCurrentPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceCurrentPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceCurrentPositionRequest): ArmSubtypeServiceCurrentPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceCurrentPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceCurrentPositionRequest;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceCurrentPositionRequest, reader: jspb.BinaryReader): ArmSubtypeServiceCurrentPositionRequest;
}

export namespace ArmSubtypeServiceCurrentPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmSubtypeServiceCurrentPositionResponse extends jspb.Message {
  hasPosition(): boolean;
  clearPosition(): void;
  getPosition(): proto_api_common_v1_common_pb.Pose | undefined;
  setPosition(value?: proto_api_common_v1_common_pb.Pose): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceCurrentPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceCurrentPositionResponse): ArmSubtypeServiceCurrentPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceCurrentPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceCurrentPositionResponse;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceCurrentPositionResponse, reader: jspb.BinaryReader): ArmSubtypeServiceCurrentPositionResponse;
}

export namespace ArmSubtypeServiceCurrentPositionResponse {
  export type AsObject = {
    position?: proto_api_common_v1_common_pb.Pose.AsObject,
  }
}

export class ArmSubtypeServiceCurrentJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceCurrentJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceCurrentJointPositionsRequest): ArmSubtypeServiceCurrentJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceCurrentJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceCurrentJointPositionsRequest;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceCurrentJointPositionsRequest, reader: jspb.BinaryReader): ArmSubtypeServiceCurrentJointPositionsRequest;
}

export namespace ArmSubtypeServiceCurrentJointPositionsRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmSubtypeServiceCurrentJointPositionsResponse extends jspb.Message {
  hasPositions(): boolean;
  clearPositions(): void;
  getPositions(): ArmJointPositions | undefined;
  setPositions(value?: ArmJointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceCurrentJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceCurrentJointPositionsResponse): ArmSubtypeServiceCurrentJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceCurrentJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceCurrentJointPositionsResponse;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceCurrentJointPositionsResponse, reader: jspb.BinaryReader): ArmSubtypeServiceCurrentJointPositionsResponse;
}

export namespace ArmSubtypeServiceCurrentJointPositionsResponse {
  export type AsObject = {
    positions?: ArmJointPositions.AsObject,
  }
}

export class ArmSubtypeServiceMoveToPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): proto_api_common_v1_common_pb.Pose | undefined;
  setTo(value?: proto_api_common_v1_common_pb.Pose): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceMoveToPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceMoveToPositionRequest): ArmSubtypeServiceMoveToPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceMoveToPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceMoveToPositionRequest;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceMoveToPositionRequest, reader: jspb.BinaryReader): ArmSubtypeServiceMoveToPositionRequest;
}

export namespace ArmSubtypeServiceMoveToPositionRequest {
  export type AsObject = {
    name: string,
    to?: proto_api_common_v1_common_pb.Pose.AsObject,
  }
}

export class ArmSubtypeServiceMoveToPositionResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceMoveToPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceMoveToPositionResponse): ArmSubtypeServiceMoveToPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceMoveToPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceMoveToPositionResponse;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceMoveToPositionResponse, reader: jspb.BinaryReader): ArmSubtypeServiceMoveToPositionResponse;
}

export namespace ArmSubtypeServiceMoveToPositionResponse {
  export type AsObject = {
  }
}

export class ArmSubtypeServiceMoveToJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTo(): boolean;
  clearTo(): void;
  getTo(): ArmJointPositions | undefined;
  setTo(value?: ArmJointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceMoveToJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceMoveToJointPositionsRequest): ArmSubtypeServiceMoveToJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceMoveToJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceMoveToJointPositionsRequest;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceMoveToJointPositionsRequest, reader: jspb.BinaryReader): ArmSubtypeServiceMoveToJointPositionsRequest;
}

export namespace ArmSubtypeServiceMoveToJointPositionsRequest {
  export type AsObject = {
    name: string,
    to?: ArmJointPositions.AsObject,
  }
}

export class ArmSubtypeServiceMoveToJointPositionsResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceMoveToJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceMoveToJointPositionsResponse): ArmSubtypeServiceMoveToJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceMoveToJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceMoveToJointPositionsResponse;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceMoveToJointPositionsResponse, reader: jspb.BinaryReader): ArmSubtypeServiceMoveToJointPositionsResponse;
}

export namespace ArmSubtypeServiceMoveToJointPositionsResponse {
  export type AsObject = {
  }
}

export class ArmSubtypeServiceJointMoveDeltaRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getJoint(): number;
  setJoint(value: number): void;

  getAmountDegs(): number;
  setAmountDegs(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceJointMoveDeltaRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceJointMoveDeltaRequest): ArmSubtypeServiceJointMoveDeltaRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceJointMoveDeltaRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceJointMoveDeltaRequest;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceJointMoveDeltaRequest, reader: jspb.BinaryReader): ArmSubtypeServiceJointMoveDeltaRequest;
}

export namespace ArmSubtypeServiceJointMoveDeltaRequest {
  export type AsObject = {
    name: string,
    joint: number,
    amountDegs: number,
  }
}

export class ArmSubtypeServiceJointMoveDeltaResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmSubtypeServiceJointMoveDeltaResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmSubtypeServiceJointMoveDeltaResponse): ArmSubtypeServiceJointMoveDeltaResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmSubtypeServiceJointMoveDeltaResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmSubtypeServiceJointMoveDeltaResponse;
  static deserializeBinaryFromReader(message: ArmSubtypeServiceJointMoveDeltaResponse, reader: jspb.BinaryReader): ArmSubtypeServiceJointMoveDeltaResponse;
}

export namespace ArmSubtypeServiceJointMoveDeltaResponse {
  export type AsObject = {
  }
}

