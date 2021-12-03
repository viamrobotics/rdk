// package: proto.api.component.v1
// file: proto/api/component/v1/camera.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_httpbody_pb from "../../../../google/api/httpbody_pb";
import * as proto_api_common_v1_common_pb from "../../../../proto/api/common/v1/common_pb";

export class CameraServiceFrameRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceFrameRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceFrameRequest): CameraServiceFrameRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceFrameRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceFrameRequest;
  static deserializeBinaryFromReader(message: CameraServiceFrameRequest, reader: jspb.BinaryReader): CameraServiceFrameRequest;
}

export namespace CameraServiceFrameRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class CameraServiceFrameResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  getFrame(): Uint8Array | string;
  getFrame_asU8(): Uint8Array;
  getFrame_asB64(): string;
  setFrame(value: Uint8Array | string): void;

  getDimX(): number;
  setDimX(value: number): void;

  getDimY(): number;
  setDimY(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceFrameResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceFrameResponse): CameraServiceFrameResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceFrameResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceFrameResponse;
  static deserializeBinaryFromReader(message: CameraServiceFrameResponse, reader: jspb.BinaryReader): CameraServiceFrameResponse;
}

export namespace CameraServiceFrameResponse {
  export type AsObject = {
    mimeType: string,
    frame: Uint8Array | string,
    dimX: number,
    dimY: number,
  }
}

export class CameraServiceRenderFrameRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceRenderFrameRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceRenderFrameRequest): CameraServiceRenderFrameRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceRenderFrameRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceRenderFrameRequest;
  static deserializeBinaryFromReader(message: CameraServiceRenderFrameRequest, reader: jspb.BinaryReader): CameraServiceRenderFrameRequest;
}

export namespace CameraServiceRenderFrameRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class CameraServicePointCloudRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServicePointCloudRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServicePointCloudRequest): CameraServicePointCloudRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServicePointCloudRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServicePointCloudRequest;
  static deserializeBinaryFromReader(message: CameraServicePointCloudRequest, reader: jspb.BinaryReader): CameraServicePointCloudRequest;
}

export namespace CameraServicePointCloudRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class CameraServicePointCloudResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  getFrame(): Uint8Array | string;
  getFrame_asU8(): Uint8Array;
  getFrame_asB64(): string;
  setFrame(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServicePointCloudResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServicePointCloudResponse): CameraServicePointCloudResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServicePointCloudResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServicePointCloudResponse;
  static deserializeBinaryFromReader(message: CameraServicePointCloudResponse, reader: jspb.BinaryReader): CameraServicePointCloudResponse;
}

export namespace CameraServicePointCloudResponse {
  export type AsObject = {
    mimeType: string,
    frame: Uint8Array | string,
  }
}

export class CameraServiceObjectPointCloudsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  getMinPointsInPlane(): number;
  setMinPointsInPlane(value: number): void;

  getMinPointsInSegment(): number;
  setMinPointsInSegment(value: number): void;

  getClusteringRadius(): number;
  setClusteringRadius(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceObjectPointCloudsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceObjectPointCloudsRequest): CameraServiceObjectPointCloudsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceObjectPointCloudsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceObjectPointCloudsRequest;
  static deserializeBinaryFromReader(message: CameraServiceObjectPointCloudsRequest, reader: jspb.BinaryReader): CameraServiceObjectPointCloudsRequest;
}

export namespace CameraServiceObjectPointCloudsRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
    minPointsInPlane: number,
    minPointsInSegment: number,
    clusteringRadius: number,
  }
}

export class CameraServiceObjectPointCloudsResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  clearFramesList(): void;
  getFramesList(): Array<Uint8Array | string>;
  getFramesList_asU8(): Array<Uint8Array>;
  getFramesList_asB64(): Array<string>;
  setFramesList(value: Array<Uint8Array | string>): void;
  addFrames(value: Uint8Array | string, index?: number): Uint8Array | string;

  clearCentersList(): void;
  getCentersList(): Array<proto_api_common_v1_common_pb.Vector3>;
  setCentersList(value: Array<proto_api_common_v1_common_pb.Vector3>): void;
  addCenters(value?: proto_api_common_v1_common_pb.Vector3, index?: number): proto_api_common_v1_common_pb.Vector3;

  clearBoundingBoxesList(): void;
  getBoundingBoxesList(): Array<proto_api_common_v1_common_pb.BoxGeometry>;
  setBoundingBoxesList(value: Array<proto_api_common_v1_common_pb.BoxGeometry>): void;
  addBoundingBoxes(value?: proto_api_common_v1_common_pb.BoxGeometry, index?: number): proto_api_common_v1_common_pb.BoxGeometry;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceObjectPointCloudsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceObjectPointCloudsResponse): CameraServiceObjectPointCloudsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceObjectPointCloudsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceObjectPointCloudsResponse;
  static deserializeBinaryFromReader(message: CameraServiceObjectPointCloudsResponse, reader: jspb.BinaryReader): CameraServiceObjectPointCloudsResponse;
}

export namespace CameraServiceObjectPointCloudsResponse {
  export type AsObject = {
    mimeType: string,
    framesList: Array<Uint8Array | string>,
    centersList: Array<proto_api_common_v1_common_pb.Vector3.AsObject>,
    boundingBoxesList: Array<proto_api_common_v1_common_pb.BoxGeometry.AsObject>,
  }
}

