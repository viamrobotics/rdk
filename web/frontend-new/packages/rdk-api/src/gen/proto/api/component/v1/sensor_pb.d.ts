// package: proto.api.component.v1
// file: proto/api/component/v1/sensor.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class SensorServiceGetReadingsRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SensorServiceGetReadingsRequest.AsObject;
  static toObject(includeInstance: boolean, msg: SensorServiceGetReadingsRequest): SensorServiceGetReadingsRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SensorServiceGetReadingsRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SensorServiceGetReadingsRequest;
  static deserializeBinaryFromReader(message: SensorServiceGetReadingsRequest, reader: jspb.BinaryReader): SensorServiceGetReadingsRequest;
}

export namespace SensorServiceGetReadingsRequest {
  export type AsObject = {
    name: string,
  }
}

export class SensorServiceGetReadingsResponse extends jspb.Message {
  clearReadingsList(): void;
  getReadingsList(): Array<google_protobuf_struct_pb.Value>;
  setReadingsList(value: Array<google_protobuf_struct_pb.Value>): void;
  addReadings(value?: google_protobuf_struct_pb.Value, index?: number): google_protobuf_struct_pb.Value;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): SensorServiceGetReadingsResponse.AsObject;
  static toObject(includeInstance: boolean, msg: SensorServiceGetReadingsResponse): SensorServiceGetReadingsResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: SensorServiceGetReadingsResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): SensorServiceGetReadingsResponse;
  static deserializeBinaryFromReader(message: SensorServiceGetReadingsResponse, reader: jspb.BinaryReader): SensorServiceGetReadingsResponse;
}

export namespace SensorServiceGetReadingsResponse {
  export type AsObject = {
    readingsList: Array<google_protobuf_struct_pb.Value.AsObject>,
  }
}

