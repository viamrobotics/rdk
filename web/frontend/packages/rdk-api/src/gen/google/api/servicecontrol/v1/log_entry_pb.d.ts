// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/log_entry.proto

import * as jspb from "google-protobuf";
import * as google_api_servicecontrol_v1_http_request_pb from "../../../../google/api/servicecontrol/v1/http_request_pb";
import * as google_logging_type_log_severity_pb from "../../../../google/logging/type/log_severity_pb";
import * as google_protobuf_any_pb from "google-protobuf/google/protobuf/any_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_protobuf_timestamp_pb from "google-protobuf/google/protobuf/timestamp_pb";

export class LogEntry extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasTimestamp(): boolean;
  clearTimestamp(): void;
  getTimestamp(): google_protobuf_timestamp_pb.Timestamp | undefined;
  setTimestamp(value?: google_protobuf_timestamp_pb.Timestamp): void;

  getSeverity(): google_logging_type_log_severity_pb.LogSeverityMap[keyof google_logging_type_log_severity_pb.LogSeverityMap];
  setSeverity(value: google_logging_type_log_severity_pb.LogSeverityMap[keyof google_logging_type_log_severity_pb.LogSeverityMap]): void;

  hasHttpRequest(): boolean;
  clearHttpRequest(): void;
  getHttpRequest(): google_api_servicecontrol_v1_http_request_pb.HttpRequest | undefined;
  setHttpRequest(value?: google_api_servicecontrol_v1_http_request_pb.HttpRequest): void;

  getTrace(): string;
  setTrace(value: string): void;

  getInsertId(): string;
  setInsertId(value: string): void;

  getLabelsMap(): jspb.Map<string, string>;
  clearLabelsMap(): void;
  hasProtoPayload(): boolean;
  clearProtoPayload(): void;
  getProtoPayload(): google_protobuf_any_pb.Any | undefined;
  setProtoPayload(value?: google_protobuf_any_pb.Any): void;

  hasTextPayload(): boolean;
  clearTextPayload(): void;
  getTextPayload(): string;
  setTextPayload(value: string): void;

  hasStructPayload(): boolean;
  clearStructPayload(): void;
  getStructPayload(): google_protobuf_struct_pb.Struct | undefined;
  setStructPayload(value?: google_protobuf_struct_pb.Struct): void;

  hasOperation(): boolean;
  clearOperation(): void;
  getOperation(): LogEntryOperation | undefined;
  setOperation(value?: LogEntryOperation): void;

  hasSourceLocation(): boolean;
  clearSourceLocation(): void;
  getSourceLocation(): LogEntrySourceLocation | undefined;
  setSourceLocation(value?: LogEntrySourceLocation): void;

  getPayloadCase(): LogEntry.PayloadCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LogEntry.AsObject;
  static toObject(includeInstance: boolean, msg: LogEntry): LogEntry.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LogEntry, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LogEntry;
  static deserializeBinaryFromReader(message: LogEntry, reader: jspb.BinaryReader): LogEntry;
}

export namespace LogEntry {
  export type AsObject = {
    name: string,
    timestamp?: google_protobuf_timestamp_pb.Timestamp.AsObject,
    severity: google_logging_type_log_severity_pb.LogSeverityMap[keyof google_logging_type_log_severity_pb.LogSeverityMap],
    httpRequest?: google_api_servicecontrol_v1_http_request_pb.HttpRequest.AsObject,
    trace: string,
    insertId: string,
    labelsMap: Array<[string, string]>,
    protoPayload?: google_protobuf_any_pb.Any.AsObject,
    textPayload: string,
    structPayload?: google_protobuf_struct_pb.Struct.AsObject,
    operation?: LogEntryOperation.AsObject,
    sourceLocation?: LogEntrySourceLocation.AsObject,
  }

  export enum PayloadCase {
    PAYLOAD_NOT_SET = 0,
    PROTO_PAYLOAD = 2,
    TEXT_PAYLOAD = 3,
    STRUCT_PAYLOAD = 6,
  }
}

export class LogEntryOperation extends jspb.Message {
  getId(): string;
  setId(value: string): void;

  getProducer(): string;
  setProducer(value: string): void;

  getFirst(): boolean;
  setFirst(value: boolean): void;

  getLast(): boolean;
  setLast(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LogEntryOperation.AsObject;
  static toObject(includeInstance: boolean, msg: LogEntryOperation): LogEntryOperation.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LogEntryOperation, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LogEntryOperation;
  static deserializeBinaryFromReader(message: LogEntryOperation, reader: jspb.BinaryReader): LogEntryOperation;
}

export namespace LogEntryOperation {
  export type AsObject = {
    id: string,
    producer: string,
    first: boolean,
    last: boolean,
  }
}

export class LogEntrySourceLocation extends jspb.Message {
  getFile(): string;
  setFile(value: string): void;

  getLine(): number;
  setLine(value: number): void;

  getFunction(): string;
  setFunction(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): LogEntrySourceLocation.AsObject;
  static toObject(includeInstance: boolean, msg: LogEntrySourceLocation): LogEntrySourceLocation.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: LogEntrySourceLocation, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): LogEntrySourceLocation;
  static deserializeBinaryFromReader(message: LogEntrySourceLocation, reader: jspb.BinaryReader): LogEntrySourceLocation;
}

export namespace LogEntrySourceLocation {
  export type AsObject = {
    file: string,
    line: number,
    pb_function: string,
  }
}

