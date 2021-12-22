// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/check_error.proto

import * as jspb from "google-protobuf";
import * as google_rpc_status_pb from "../../../../google/rpc/status_pb";

export class CheckError extends jspb.Message {
  getCode(): CheckError.CodeMap[keyof CheckError.CodeMap];
  setCode(value: CheckError.CodeMap[keyof CheckError.CodeMap]): void;

  getSubject(): string;
  setSubject(value: string): void;

  getDetail(): string;
  setDetail(value: string): void;

  hasStatus(): boolean;
  clearStatus(): void;
  getStatus(): google_rpc_status_pb.Status | undefined;
  setStatus(value?: google_rpc_status_pb.Status): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckError.AsObject;
  static toObject(includeInstance: boolean, msg: CheckError): CheckError.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckError, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckError;
  static deserializeBinaryFromReader(message: CheckError, reader: jspb.BinaryReader): CheckError;
}

export namespace CheckError {
  export type AsObject = {
    code: CheckError.CodeMap[keyof CheckError.CodeMap],
    subject: string,
    detail: string,
    status?: google_rpc_status_pb.Status.AsObject,
  }

  export interface CodeMap {
    ERROR_CODE_UNSPECIFIED: 0;
    NOT_FOUND: 5;
    PERMISSION_DENIED: 7;
    RESOURCE_EXHAUSTED: 8;
    SERVICE_NOT_ACTIVATED: 104;
    BILLING_DISABLED: 107;
    PROJECT_DELETED: 108;
    PROJECT_INVALID: 114;
    CONSUMER_INVALID: 125;
    IP_ADDRESS_BLOCKED: 109;
    REFERER_BLOCKED: 110;
    CLIENT_APP_BLOCKED: 111;
    API_TARGET_BLOCKED: 122;
    API_KEY_INVALID: 105;
    API_KEY_EXPIRED: 112;
    API_KEY_NOT_FOUND: 113;
    INVALID_CREDENTIAL: 123;
    NAMESPACE_LOOKUP_UNAVAILABLE: 300;
    SERVICE_STATUS_UNAVAILABLE: 301;
    BILLING_STATUS_UNAVAILABLE: 302;
    CLOUD_RESOURCE_MANAGER_BACKEND_UNAVAILABLE: 305;
  }

  export const Code: CodeMap;
}

