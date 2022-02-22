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

export class ArmServiceGetEndPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceGetEndPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceGetEndPositionRequest): ArmServiceGetEndPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceGetEndPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceGetEndPositionRequest;
  static deserializeBinaryFromReader(message: ArmServiceGetEndPositionRequest, reader: jspb.BinaryReader): ArmServiceGetEndPositionRequest;
}

export namespace ArmServiceGetEndPositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmServiceGetEndPositionResponse extends jspb.Message {
  hasPose(): boolean;
  clearPose(): void;
  getPose(): proto_api_common_v1_common_pb.Pose | undefined;
  setPose(value?: proto_api_common_v1_common_pb.Pose): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceGetEndPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceGetEndPositionResponse): ArmServiceGetEndPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceGetEndPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceGetEndPositionResponse;
  static deserializeBinaryFromReader(message: ArmServiceGetEndPositionResponse, reader: jspb.BinaryReader): ArmServiceGetEndPositionResponse;
}

export namespace ArmServiceGetEndPositionResponse {
  export type AsObject = {
    pose?: proto_api_common_v1_common_pb.Pose.AsObject,
  }
}

export class ArmServiceGetJointPositionsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceGetJointPositionsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceGetJointPositionsRequest): ArmServiceGetJointPositionsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceGetJointPositionsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceGetJointPositionsRequest;
  static deserializeBinaryFromReader(message: ArmServiceGetJointPositionsRequest, reader: jspb.BinaryReader): ArmServiceGetJointPositionsRequest;
}

export namespace ArmServiceGetJointPositionsRequest {
  export type AsObject = {
    name: string,
  }
}

export class ArmServiceGetJointPositionsResponse extends jspb.Message {
  hasPositionDegs(): boolean;
  clearPositionDegs(): void;
  getPositionDegs(): ArmJointPositions | undefined;
  setPositionDegs(value?: ArmJointPositions): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ArmServiceGetJointPositionsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ArmServiceGetJointPositionsResponse): ArmServiceGetJointPositionsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ArmServiceGetJointPositionsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ArmServiceGetJointPositionsResponse;
  static deserializeBinaryFromReader(message: ArmServiceGetJointPositionsResponse, reader: jspb.BinaryReader): ArmServiceGetJointPositionsResponse;
}

export namespace ArmServiceGetJointPositionsResponse {
  export type AsObject = {
    positionDegs?: ArmJointPositions.AsObject,
  }
}

export class ArmServiceMoveToPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasPose(): boolean;
  clearPose(): void;
  getPose(): proto_api_common_v1_common_pb.Pose | undefined;
  setPose(value?: proto_api_common_v1_common_pb.Pose): void;

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
    pose?: proto_api_common_v1_common_pb.Pose.AsObject,
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

  hasPositionDegs(): boolean;
  clearPositionDegs(): void;
  getPositionDegs(): ArmJointPositions | undefined;
  setPositionDegs(value?: ArmJointPositions): void;

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
    positionDegs?: ArmJointPositions.AsObject,
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

