// package: google.api.expr.conformance.v1alpha1
// file: google/api/expr/conformance/v1alpha1/conformance_service.proto

import * as jspb from "google-protobuf";
import * as google_api_expr_v1alpha1_checked_pb from "../../../../../google/api/expr/v1alpha1/checked_pb";
import * as google_api_expr_v1alpha1_eval_pb from "../../../../../google/api/expr/v1alpha1/eval_pb";
import * as google_api_expr_v1alpha1_syntax_pb from "../../../../../google/api/expr/v1alpha1/syntax_pb";
import * as google_rpc_status_pb from "../../../../../google/rpc/status_pb";
import * as google_api_client_pb from "../../../../../google/api/client_pb";

export class ParseRequest extends jspb.Message {
  getCelSource(): string;
  setCelSource(value: string): void;

  getSyntaxVersion(): string;
  setSyntaxVersion(value: string): void;

  getSourceLocation(): string;
  setSourceLocation(value: string): void;

  getDisableMacros(): boolean;
  setDisableMacros(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ParseRequest.AsObject;
  static toObject(includeInstance: boolean, msg: ParseRequest): ParseRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ParseRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ParseRequest;
  static deserializeBinaryFromReader(message: ParseRequest, reader: jspb.BinaryReader): ParseRequest;
}

export namespace ParseRequest {
  export type AsObject = {
    celSource: string,
    syntaxVersion: string,
    sourceLocation: string,
    disableMacros: boolean,
  }
}

export class ParseResponse extends jspb.Message {
  hasParsedExpr(): boolean;
  clearParsedExpr(): void;
  getParsedExpr(): google_api_expr_v1alpha1_syntax_pb.ParsedExpr | undefined;
  setParsedExpr(value?: google_api_expr_v1alpha1_syntax_pb.ParsedExpr): void;

  clearIssuesList(): void;
  getIssuesList(): Array<google_rpc_status_pb.Status>;
  setIssuesList(value: Array<google_rpc_status_pb.Status>): void;
  addIssues(value?: google_rpc_status_pb.Status, index?: number): google_rpc_status_pb.Status;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ParseResponse.AsObject;
  static toObject(includeInstance: boolean, msg: ParseResponse): ParseResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ParseResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ParseResponse;
  static deserializeBinaryFromReader(message: ParseResponse, reader: jspb.BinaryReader): ParseResponse;
}

export namespace ParseResponse {
  export type AsObject = {
    parsedExpr?: google_api_expr_v1alpha1_syntax_pb.ParsedExpr.AsObject,
    issuesList: Array<google_rpc_status_pb.Status.AsObject>,
  }
}

export class CheckRequest extends jspb.Message {
  hasParsedExpr(): boolean;
  clearParsedExpr(): void;
  getParsedExpr(): google_api_expr_v1alpha1_syntax_pb.ParsedExpr | undefined;
  setParsedExpr(value?: google_api_expr_v1alpha1_syntax_pb.ParsedExpr): void;

  clearTypeEnvList(): void;
  getTypeEnvList(): Array<google_api_expr_v1alpha1_checked_pb.Decl>;
  setTypeEnvList(value: Array<google_api_expr_v1alpha1_checked_pb.Decl>): void;
  addTypeEnv(value?: google_api_expr_v1alpha1_checked_pb.Decl, index?: number): google_api_expr_v1alpha1_checked_pb.Decl;

  getContainer(): string;
  setContainer(value: string): void;

  getNoStdEnv(): boolean;
  setNoStdEnv(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckRequest.AsObject;
  static toObject(includeInstance: boolean, msg: CheckRequest): CheckRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckRequest;
  static deserializeBinaryFromReader(message: CheckRequest, reader: jspb.BinaryReader): CheckRequest;
}

export namespace CheckRequest {
  export type AsObject = {
    parsedExpr?: google_api_expr_v1alpha1_syntax_pb.ParsedExpr.AsObject,
    typeEnvList: Array<google_api_expr_v1alpha1_checked_pb.Decl.AsObject>,
    container: string,
    noStdEnv: boolean,
  }
}

export class CheckResponse extends jspb.Message {
  hasCheckedExpr(): boolean;
  clearCheckedExpr(): void;
  getCheckedExpr(): google_api_expr_v1alpha1_checked_pb.CheckedExpr | undefined;
  setCheckedExpr(value?: google_api_expr_v1alpha1_checked_pb.CheckedExpr): void;

  clearIssuesList(): void;
  getIssuesList(): Array<google_rpc_status_pb.Status>;
  setIssuesList(value: Array<google_rpc_status_pb.Status>): void;
  addIssues(value?: google_rpc_status_pb.Status, index?: number): google_rpc_status_pb.Status;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckResponse.AsObject;
  static toObject(includeInstance: boolean, msg: CheckResponse): CheckResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckResponse;
  static deserializeBinaryFromReader(message: CheckResponse, reader: jspb.BinaryReader): CheckResponse;
}

export namespace CheckResponse {
  export type AsObject = {
    checkedExpr?: google_api_expr_v1alpha1_checked_pb.CheckedExpr.AsObject,
    issuesList: Array<google_rpc_status_pb.Status.AsObject>,
  }
}

export class EvalRequest extends jspb.Message {
  hasParsedExpr(): boolean;
  clearParsedExpr(): void;
  getParsedExpr(): google_api_expr_v1alpha1_syntax_pb.ParsedExpr | undefined;
  setParsedExpr(value?: google_api_expr_v1alpha1_syntax_pb.ParsedExpr): void;

  hasCheckedExpr(): boolean;
  clearCheckedExpr(): void;
  getCheckedExpr(): google_api_expr_v1alpha1_checked_pb.CheckedExpr | undefined;
  setCheckedExpr(value?: google_api_expr_v1alpha1_checked_pb.CheckedExpr): void;

  getBindingsMap(): jspb.Map<string, google_api_expr_v1alpha1_eval_pb.ExprValue>;
  clearBindingsMap(): void;
  getContainer(): string;
  setContainer(value: string): void;

  getExprKindCase(): EvalRequest.ExprKindCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EvalRequest.AsObject;
  static toObject(includeInstance: boolean, msg: EvalRequest): EvalRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EvalRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EvalRequest;
  static deserializeBinaryFromReader(message: EvalRequest, reader: jspb.BinaryReader): EvalRequest;
}

export namespace EvalRequest {
  export type AsObject = {
    parsedExpr?: google_api_expr_v1alpha1_syntax_pb.ParsedExpr.AsObject,
    checkedExpr?: google_api_expr_v1alpha1_checked_pb.CheckedExpr.AsObject,
    bindingsMap: Array<[string, google_api_expr_v1alpha1_eval_pb.ExprValue.AsObject]>,
    container: string,
  }

  export enum ExprKindCase {
    EXPR_KIND_NOT_SET = 0,
    PARSED_EXPR = 1,
    CHECKED_EXPR = 2,
  }
}

export class EvalResponse extends jspb.Message {
  hasResult(): boolean;
  clearResult(): void;
  getResult(): google_api_expr_v1alpha1_eval_pb.ExprValue | undefined;
  setResult(value?: google_api_expr_v1alpha1_eval_pb.ExprValue): void;

  clearIssuesList(): void;
  getIssuesList(): Array<google_rpc_status_pb.Status>;
  setIssuesList(value: Array<google_rpc_status_pb.Status>): void;
  addIssues(value?: google_rpc_status_pb.Status, index?: number): google_rpc_status_pb.Status;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EvalResponse.AsObject;
  static toObject(includeInstance: boolean, msg: EvalResponse): EvalResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EvalResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EvalResponse;
  static deserializeBinaryFromReader(message: EvalResponse, reader: jspb.BinaryReader): EvalResponse;
}

export namespace EvalResponse {
  export type AsObject = {
    result?: google_api_expr_v1alpha1_eval_pb.ExprValue.AsObject,
    issuesList: Array<google_rpc_status_pb.Status.AsObject>,
  }
}

export class IssueDetails extends jspb.Message {
  getSeverity(): IssueDetails.SeverityMap[keyof IssueDetails.SeverityMap];
  setSeverity(value: IssueDetails.SeverityMap[keyof IssueDetails.SeverityMap]): void;

  hasPosition(): boolean;
  clearPosition(): void;
  getPosition(): google_api_expr_v1alpha1_syntax_pb.SourcePosition | undefined;
  setPosition(value?: google_api_expr_v1alpha1_syntax_pb.SourcePosition): void;

  getId(): number;
  setId(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IssueDetails.AsObject;
  static toObject(includeInstance: boolean, msg: IssueDetails): IssueDetails.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IssueDetails, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IssueDetails;
  static deserializeBinaryFromReader(message: IssueDetails, reader: jspb.BinaryReader): IssueDetails;
}

export namespace IssueDetails {
  export type AsObject = {
    severity: IssueDetails.SeverityMap[keyof IssueDetails.SeverityMap],
    position?: google_api_expr_v1alpha1_syntax_pb.SourcePosition.AsObject,
    id: number,
  }

  export interface SeverityMap {
    SEVERITY_UNSPECIFIED: 0;
    DEPRECATION: 1;
    WARNING: 2;
    ERROR: 3;
  }

  export const Severity: SeverityMap;
}

