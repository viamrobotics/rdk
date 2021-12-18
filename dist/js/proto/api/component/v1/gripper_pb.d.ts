// package: proto.api.component.v1
// file: proto/api/component/v1/gripper.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class GripperServiceOpenRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperServiceOpenRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GripperServiceOpenRequest): GripperServiceOpenRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperServiceOpenRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperServiceOpenRequest;
  static deserializeBinaryFromReader(message: GripperServiceOpenRequest, reader: jspb.BinaryReader): GripperServiceOpenRequest;
}

export namespace GripperServiceOpenRequest {
  export type AsObject = {
    name: string,
  }
}

export class GripperServiceOpenResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperServiceOpenResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GripperServiceOpenResponse): GripperServiceOpenResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperServiceOpenResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperServiceOpenResponse;
  static deserializeBinaryFromReader(message: GripperServiceOpenResponse, reader: jspb.BinaryReader): GripperServiceOpenResponse;
}

export namespace GripperServiceOpenResponse {
  export type AsObject = {
  }
}

export class GripperServiceGrabRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperServiceGrabRequest.AsObject;
  static toObject(includeInstance: boolean, msg: GripperServiceGrabRequest): GripperServiceGrabRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperServiceGrabRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperServiceGrabRequest;
  static deserializeBinaryFromReader(message: GripperServiceGrabRequest, reader: jspb.BinaryReader): GripperServiceGrabRequest;
}

export namespace GripperServiceGrabRequest {
  export type AsObject = {
    name: string,
  }
}

export class GripperServiceGrabResponse extends jspb.Message {
  getGrabbed(): boolean;
  setGrabbed(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): GripperServiceGrabResponse.AsObject;
  static toObject(includeInstance: boolean, msg: GripperServiceGrabResponse): GripperServiceGrabResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: GripperServiceGrabResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): GripperServiceGrabResponse;
  static deserializeBinaryFromReader(message: GripperServiceGrabResponse, reader: jspb.BinaryReader): GripperServiceGrabResponse;
}

export namespace GripperServiceGrabResponse {
  export type AsObject = {
    grabbed: boolean,
  }
}

