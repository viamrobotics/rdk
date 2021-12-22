// package: google.api.expr.v1beta1
// file: google/api/expr/v1beta1/eval.proto

import * as jspb from "google-protobuf";
import * as google_api_expr_v1beta1_value_pb from "../../../../google/api/expr/v1beta1/value_pb";
import * as google_rpc_status_pb from "../../../../google/rpc/status_pb";

export class EvalState extends jspb.Message {
  clearValuesList(): void;
  getValuesList(): Array<ExprValue>;
  setValuesList(value: Array<ExprValue>): void;
  addValues(value?: ExprValue, index?: number): ExprValue;

  clearResultsList(): void;
  getResultsList(): Array<EvalState.Result>;
  setResultsList(value: Array<EvalState.Result>): void;
  addResults(value?: EvalState.Result, index?: number): EvalState.Result;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): EvalState.AsObject;
  static toObject(includeInstance: boolean, msg: EvalState): EvalState.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: EvalState, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): EvalState;
  static deserializeBinaryFromReader(message: EvalState, reader: jspb.BinaryReader): EvalState;
}

export namespace EvalState {
  export type AsObject = {
    valuesList: Array<ExprValue.AsObject>,
    resultsList: Array<EvalState.Result.AsObject>,
  }

  export class Result extends jspb.Message {
    hasExpr(): boolean;
    clearExpr(): void;
    getExpr(): IdRef | undefined;
    setExpr(value?: IdRef): void;

    getValue(): number;
    setValue(value: number): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Result.AsObject;
    static toObject(includeInstance: boolean, msg: Result): Result.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Result, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Result;
    static deserializeBinaryFromReader(message: Result, reader: jspb.BinaryReader): Result;
  }

  export namespace Result {
    export type AsObject = {
      expr?: IdRef.AsObject,
      value: number,
    }
  }
}

export class ExprValue extends jspb.Message {
  hasValue(): boolean;
  clearValue(): void;
  getValue(): google_api_expr_v1beta1_value_pb.Value | undefined;
  setValue(value?: google_api_expr_v1beta1_value_pb.Value): void;

  hasError(): boolean;
  clearError(): void;
  getError(): ErrorSet | undefined;
  setError(value?: ErrorSet): void;

  hasUnknown(): boolean;
  clearUnknown(): void;
  getUnknown(): UnknownSet | undefined;
  setUnknown(value?: UnknownSet): void;

  getKindCase(): ExprValue.KindCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ExprValue.AsObject;
  static toObject(includeInstance: boolean, msg: ExprValue): ExprValue.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ExprValue, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ExprValue;
  static deserializeBinaryFromReader(message: ExprValue, reader: jspb.BinaryReader): ExprValue;
}

export namespace ExprValue {
  export type AsObject = {
    value?: google_api_expr_v1beta1_value_pb.Value.AsObject,
    error?: ErrorSet.AsObject,
    unknown?: UnknownSet.AsObject,
  }

  export enum KindCase {
    KIND_NOT_SET = 0,
    VALUE = 1,
    ERROR = 2,
    UNKNOWN = 3,
  }
}

export class ErrorSet extends jspb.Message {
  clearErrorsList(): void;
  getErrorsList(): Array<google_rpc_status_pb.Status>;
  setErrorsList(value: Array<google_rpc_status_pb.Status>): void;
  addErrors(value?: google_rpc_status_pb.Status, index?: number): google_rpc_status_pb.Status;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ErrorSet.AsObject;
  static toObject(includeInstance: boolean, msg: ErrorSet): ErrorSet.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ErrorSet, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ErrorSet;
  static deserializeBinaryFromReader(message: ErrorSet, reader: jspb.BinaryReader): ErrorSet;
}

export namespace ErrorSet {
  export type AsObject = {
    errorsList: Array<google_rpc_status_pb.Status.AsObject>,
  }
}

export class UnknownSet extends jspb.Message {
  clearExprsList(): void;
  getExprsList(): Array<IdRef>;
  setExprsList(value: Array<IdRef>): void;
  addExprs(value?: IdRef, index?: number): IdRef;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): UnknownSet.AsObject;
  static toObject(includeInstance: boolean, msg: UnknownSet): UnknownSet.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: UnknownSet, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): UnknownSet;
  static deserializeBinaryFromReader(message: UnknownSet, reader: jspb.BinaryReader): UnknownSet;
}

export namespace UnknownSet {
  export type AsObject = {
    exprsList: Array<IdRef.AsObject>,
  }
}

export class IdRef extends jspb.Message {
  getId(): number;
  setId(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): IdRef.AsObject;
  static toObject(includeInstance: boolean, msg: IdRef): IdRef.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: IdRef, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): IdRef;
  static deserializeBinaryFromReader(message: IdRef, reader: jspb.BinaryReader): IdRef;
}

export namespace IdRef {
  export type AsObject = {
    id: number,
  }
}

