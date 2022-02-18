// package: google.api.expr.v1beta1
// file: google/api/expr/v1beta1/source.proto

import * as jspb from "google-protobuf";

export class SourceInfo extends jspb.Message {
  getLocation(): string;
  setLocation(value: string): void;

  clearLineOffsetsList(): void;
  getLineOffsetsList(): Array<number>;
  setLineOffsetsList(value: Array<number>): void;
  addLineOffsets(value: number, index?: number): number;

  getPositionsMap(): jspb.Map<number, number>;
  clearPositionsMap(): void;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SourceInfo.AsObject;
  static toObject(includeInstance: boolean, msg: SourceInfo): SourceInfo.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SourceInfo, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SourceInfo;
  static deserializeBinaryFromReader(message: SourceInfo, reader: jspb.BinaryReader): SourceInfo;
}

export namespace SourceInfo {
  export type AsObject = {
    location: string,
    lineOffsetsList: Array<number>,
    positionsMap: Array<[number, number]>,
  }
}

export class SourcePosition extends jspb.Message {
  getLocation(): string;
  setLocation(value: string): void;

  getOffset(): number;
  setOffset(value: number): void;

  getLine(): number;
  setLine(value: number): void;

  getColumn(): number;
  setColumn(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SourcePosition.AsObject;
  static toObject(includeInstance: boolean, msg: SourcePosition): SourcePosition.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SourcePosition, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SourcePosition;
  static deserializeBinaryFromReader(message: SourcePosition, reader: jspb.BinaryReader): SourcePosition;
}

export namespace SourcePosition {
  export type AsObject = {
    location: string,
    offset: number,
    line: number,
    column: number,
  }
}

