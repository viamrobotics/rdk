// package: proto.api.component.v1
// file: proto/api/component/v1/forcematrix.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class Matrix extends jspb.Message {
  getRows(): number;
  setRows(value: number): void;

  getCols(): number;
  setCols(value: number): void;

  clearDataList(): void;
  getDataList(): Array<number>;
  setDataList(value: Array<number>): void;
  addData(value: number, index?: number): number;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Matrix.AsObject;
  static toObject(includeInstance: boolean, msg: Matrix): Matrix.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Matrix, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Matrix;
  static deserializeBinaryFromReader(message: Matrix, reader: jspb.BinaryReader): Matrix;
}

export namespace Matrix {
  export type AsObject = {
    rows: number,
    cols: number,
    dataList: Array<number>,
  }
}

export class ForceMatrixServiceReadMatrixRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ForceMatrixServiceReadMatrixRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ForceMatrixServiceReadMatrixRequest): ForceMatrixServiceReadMatrixRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ForceMatrixServiceReadMatrixRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ForceMatrixServiceReadMatrixRequest;
  static deserializeBinaryFromReader(message: ForceMatrixServiceReadMatrixRequest, reader: jspb.BinaryReader): ForceMatrixServiceReadMatrixRequest;
}

export namespace ForceMatrixServiceReadMatrixRequest {
  export type AsObject = {
    name: string,
  }
}

export class ForceMatrixServiceReadMatrixResponse extends jspb.Message {
  hasMatrix(): boolean;
  clearMatrix(): void;
  getMatrix(): Matrix | undefined;
  setMatrix(value?: Matrix): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ForceMatrixServiceReadMatrixResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ForceMatrixServiceReadMatrixResponse): ForceMatrixServiceReadMatrixResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ForceMatrixServiceReadMatrixResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ForceMatrixServiceReadMatrixResponse;
  static deserializeBinaryFromReader(message: ForceMatrixServiceReadMatrixResponse, reader: jspb.BinaryReader): ForceMatrixServiceReadMatrixResponse;
}

export namespace ForceMatrixServiceReadMatrixResponse {
  export type AsObject = {
    matrix?: Matrix.AsObject,
  }
}

export class ForceMatrixServiceDetectSlipRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ForceMatrixServiceDetectSlipRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ForceMatrixServiceDetectSlipRequest): ForceMatrixServiceDetectSlipRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ForceMatrixServiceDetectSlipRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ForceMatrixServiceDetectSlipRequest;
  static deserializeBinaryFromReader(message: ForceMatrixServiceDetectSlipRequest, reader: jspb.BinaryReader): ForceMatrixServiceDetectSlipRequest;
}

export namespace ForceMatrixServiceDetectSlipRequest {
  export type AsObject = {
    name: string,
  }
}

export class ForceMatrixServiceDetectSlipResponse extends jspb.Message {
  getSlipDetected(): boolean;
  setSlipDetected(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ForceMatrixServiceDetectSlipResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ForceMatrixServiceDetectSlipResponse): ForceMatrixServiceDetectSlipResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ForceMatrixServiceDetectSlipResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ForceMatrixServiceDetectSlipResponse;
  static deserializeBinaryFromReader(message: ForceMatrixServiceDetectSlipResponse, reader: jspb.BinaryReader): ForceMatrixServiceDetectSlipResponse;
}

export namespace ForceMatrixServiceDetectSlipResponse {
  export type AsObject = {
    slipDetected: boolean,
  }
}

