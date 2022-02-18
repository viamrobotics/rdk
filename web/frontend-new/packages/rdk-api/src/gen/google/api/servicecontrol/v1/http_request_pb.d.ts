// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/http_request.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_duration_pb from "google-protobuf/google/protobuf/duration_pb";

export class HttpRequest extends jspb.Message {
  getRequestMethod(): string;
  setRequestMethod(value: string): void;

  getRequestUrl(): string;
  setRequestUrl(value: string): void;

  getRequestSize(): number;
  setRequestSize(value: number): void;

  getStatus(): number;
  setStatus(value: number): void;

  getResponseSize(): number;
  setResponseSize(value: number): void;

  getUserAgent(): string;
  setUserAgent(value: string): void;

  getRemoteIp(): string;
  setRemoteIp(value: string): void;

  getServerIp(): string;
  setServerIp(value: string): void;

  getReferer(): string;
  setReferer(value: string): void;

  hasLatency(): boolean;
  clearLatency(): void;
  getLatency(): google_protobuf_duration_pb.Duration | undefined;
  setLatency(value?: google_protobuf_duration_pb.Duration): void;

  getCacheLookup(): boolean;
  setCacheLookup(value: boolean): void;

  getCacheHit(): boolean;
  setCacheHit(value: boolean): void;

  getCacheValidatedWithOriginServer(): boolean;
  setCacheValidatedWithOriginServer(value: boolean): void;

  getCacheFillBytes(): number;
  setCacheFillBytes(value: number): void;

  getProtocol(): string;
  setProtocol(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): HttpRequest.AsObject;
  static toObject(includeInstance: boolean, msg: HttpRequest): HttpRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: HttpRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): HttpRequest;
  static deserializeBinaryFromReader(message: HttpRequest, reader: jspb.BinaryReader): HttpRequest;
}

export namespace HttpRequest {
  export type AsObject = {
    requestMethod: string,
    requestUrl: string,
    requestSize: number,
    status: number,
    responseSize: number,
    userAgent: string,
    remoteIp: string,
    serverIp: string,
    referer: string,
    latency?: google_protobuf_duration_pb.Duration.AsObject,
    cacheLookup: boolean,
    cacheHit: boolean,
    cacheValidatedWithOriginServer: boolean,
    cacheFillBytes: number,
    protocol: string,
  }
}

