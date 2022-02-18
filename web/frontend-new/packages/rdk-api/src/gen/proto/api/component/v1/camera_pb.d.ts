// package: proto.api.component.v1
// file: proto/api/component/v1/camera.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as google_api_httpbody_pb from "../../../../google/api/httpbody_pb";
import * as proto_api_common_v1_common_pb from "../../../../proto/api/common/v1/common_pb";

export class CameraServiceGetFrameRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceGetFrameRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceGetFrameRequest): CameraServiceGetFrameRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceGetFrameRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceGetFrameRequest;
  static deserializeBinaryFromReader(message: CameraServiceGetFrameRequest, reader: jspb.BinaryReader): CameraServiceGetFrameRequest;
}

export namespace CameraServiceGetFrameRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class CameraServiceGetFrameResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  getFrame(): Uint8Array | string;
  getFrame_asU8(): Uint8Array;
  getFrame_asB64(): string;
  setFrame(value: Uint8Array | string): void;

  getWidthPx(): number;
  setWidthPx(value: number): void;

  getHeightPx(): number;
  setHeightPx(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceGetFrameResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceGetFrameResponse): CameraServiceGetFrameResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceGetFrameResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceGetFrameResponse;
  static deserializeBinaryFromReader(message: CameraServiceGetFrameResponse, reader: jspb.BinaryReader): CameraServiceGetFrameResponse;
}

export namespace CameraServiceGetFrameResponse {
  export type AsObject = {
    mimeType: string,
    frame: Uint8Array | string,
    widthPx: number,
    heightPx: number,
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

export class CameraServiceGetPointCloudRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceGetPointCloudRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceGetPointCloudRequest): CameraServiceGetPointCloudRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceGetPointCloudRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceGetPointCloudRequest;
  static deserializeBinaryFromReader(message: CameraServiceGetPointCloudRequest, reader: jspb.BinaryReader): CameraServiceGetPointCloudRequest;
}

export namespace CameraServiceGetPointCloudRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
  }
}

export class CameraServiceGetPointCloudResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  getFrame(): Uint8Array | string;
  getFrame_asU8(): Uint8Array;
  getFrame_asB64(): string;
  setFrame(value: Uint8Array | string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceGetPointCloudResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceGetPointCloudResponse): CameraServiceGetPointCloudResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceGetPointCloudResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceGetPointCloudResponse;
  static deserializeBinaryFromReader(message: CameraServiceGetPointCloudResponse, reader: jspb.BinaryReader): CameraServiceGetPointCloudResponse;
}

export namespace CameraServiceGetPointCloudResponse {
  export type AsObject = {
    mimeType: string,
    frame: Uint8Array | string,
  }
}

export class CameraServiceGetObjectPointCloudsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getMimeType(): string;
  setMimeType(value: string): void;

  getMinPointsInPlane(): number;
  setMinPointsInPlane(value: number): void;

  getMinPointsInSegment(): number;
  setMinPointsInSegment(value: number): void;

  getClusteringRadiusMm(): number;
  setClusteringRadiusMm(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceGetObjectPointCloudsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceGetObjectPointCloudsRequest): CameraServiceGetObjectPointCloudsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceGetObjectPointCloudsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceGetObjectPointCloudsRequest;
  static deserializeBinaryFromReader(message: CameraServiceGetObjectPointCloudsRequest, reader: jspb.BinaryReader): CameraServiceGetObjectPointCloudsRequest;
}

export namespace CameraServiceGetObjectPointCloudsRequest {
  export type AsObject = {
    name: string,
    mimeType: string,
    minPointsInPlane: number,
    minPointsInSegment: number,
    clusteringRadiusMm: number,
  }
}

export class CameraServiceGetObjectPointCloudsResponse extends jspb.Message {
  getMimeType(): string;
  setMimeType(value: string): void;

  clearObjectsList(): void;
  getObjectsList(): Array<PointCloudObject>;
  setObjectsList(value: Array<PointCloudObject>): void;
  addObjects(value?: PointCloudObject, index?: number): PointCloudObject;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CameraServiceGetObjectPointCloudsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CameraServiceGetObjectPointCloudsResponse): CameraServiceGetObjectPointCloudsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CameraServiceGetObjectPointCloudsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CameraServiceGetObjectPointCloudsResponse;
  static deserializeBinaryFromReader(message: CameraServiceGetObjectPointCloudsResponse, reader: jspb.BinaryReader): CameraServiceGetObjectPointCloudsResponse;
}

export namespace CameraServiceGetObjectPointCloudsResponse {
  export type AsObject = {
    mimeType: string,
    objectsList: Array<PointCloudObject.AsObject>,
  }
}

export class PointCloudObject extends jspb.Message {
  getFrame(): Uint8Array | string;
  getFrame_asU8(): Uint8Array;
  getFrame_asB64(): string;
  setFrame(value: Uint8Array | string): void;

  hasCenterCoordinatesMm(): boolean;
  clearCenterCoordinatesMm(): void;
  getCenterCoordinatesMm(): proto_api_common_v1_common_pb.Vector3 | undefined;
  setCenterCoordinatesMm(value?: proto_api_common_v1_common_pb.Vector3): void;

  hasBoundingBoxMm(): boolean;
  clearBoundingBoxMm(): void;
  getBoundingBoxMm(): proto_api_common_v1_common_pb.BoxGeometry | undefined;
  setBoundingBoxMm(value?: proto_api_common_v1_common_pb.BoxGeometry): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): PointCloudObject.AsObject;
  static toObject(includeInstance: boolean, msg: PointCloudObject): PointCloudObject.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: PointCloudObject, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): PointCloudObject;
  static deserializeBinaryFromReader(message: PointCloudObject, reader: jspb.BinaryReader): PointCloudObject;
}

export namespace PointCloudObject {
  export type AsObject = {
    frame: Uint8Array | string,
    centerCoordinatesMm?: proto_api_common_v1_common_pb.Vector3.AsObject,
    boundingBoxMm?: proto_api_common_v1_common_pb.BoxGeometry.AsObject,
  }
}

