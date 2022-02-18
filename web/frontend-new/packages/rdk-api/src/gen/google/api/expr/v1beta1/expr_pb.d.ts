// package: google.api.expr.v1beta1
// file: google/api/expr/v1beta1/expr.proto

import * as jspb from "google-protobuf";
import * as google_api_expr_v1beta1_source_pb from "../../../../google/api/expr/v1beta1/source_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class ParsedExpr extends jspb.Message {
  hasExpr(): boolean;
  clearExpr(): void;
  getExpr(): Expr | undefined;
  setExpr(value?: Expr): void;

  hasSourceInfo(): boolean;
  clearSourceInfo(): void;
  getSourceInfo(): google_api_expr_v1beta1_source_pb.SourceInfo | undefined;
  setSourceInfo(value?: google_api_expr_v1beta1_source_pb.SourceInfo): void;

  getSyntaxVersion(): string;
  setSyntaxVersion(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): ParsedExpr.AsObject;
  static toObject(includeInstance: boolean, msg: ParsedExpr): ParsedExpr.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: ParsedExpr, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): ParsedExpr;
  static deserializeBinaryFromReader(message: ParsedExpr, reader: jspb.BinaryReader): ParsedExpr;
}

export namespace ParsedExpr {
  export type AsObject = {
    expr?: Expr.AsObject,
    sourceInfo?: google_api_expr_v1beta1_source_pb.SourceInfo.AsObject,
    syntaxVersion: string,
  }
}

export class Expr extends jspb.Message {
  getId(): number;
  setId(value: number): void;

  hasLiteralExpr(): boolean;
  clearLiteralExpr(): void;
  getLiteralExpr(): Literal | undefined;
  setLiteralExpr(value?: Literal): void;

  hasIdentExpr(): boolean;
  clearIdentExpr(): void;
  getIdentExpr(): Expr.Ident | undefined;
  setIdentExpr(value?: Expr.Ident): void;

  hasSelectExpr(): boolean;
  clearSelectExpr(): void;
  getSelectExpr(): Expr.Select | undefined;
  setSelectExpr(value?: Expr.Select): void;

  hasCallExpr(): boolean;
  clearCallExpr(): void;
  getCallExpr(): Expr.Call | undefined;
  setCallExpr(value?: Expr.Call): void;

  hasListExpr(): boolean;
  clearListExpr(): void;
  getListExpr(): Expr.CreateList | undefined;
  setListExpr(value?: Expr.CreateList): void;

  hasStructExpr(): boolean;
  clearStructExpr(): void;
  getStructExpr(): Expr.CreateStruct | undefined;
  setStructExpr(value?: Expr.CreateStruct): void;

  hasComprehensionExpr(): boolean;
  clearComprehensionExpr(): void;
  getComprehensionExpr(): Expr.Comprehension | undefined;
  setComprehensionExpr(value?: Expr.Comprehension): void;

  getExprKindCase(): Expr.ExprKindCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Expr.AsObject;
  static toObject(includeInstance: boolean, msg: Expr): Expr.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Expr, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Expr;
  static deserializeBinaryFromReader(message: Expr, reader: jspb.BinaryReader): Expr;
}

export namespace Expr {
  export type AsObject = {
    id: number,
    literalExpr?: Literal.AsObject,
    identExpr?: Expr.Ident.AsObject,
    selectExpr?: Expr.Select.AsObject,
    callExpr?: Expr.Call.AsObject,
    listExpr?: Expr.CreateList.AsObject,
    structExpr?: Expr.CreateStruct.AsObject,
    comprehensionExpr?: Expr.Comprehension.AsObject,
  }

  export class Ident extends jspb.Message {
    getName(): string;
    setName(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Ident.AsObject;
    static toObject(includeInstance: boolean, msg: Ident): Ident.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Ident, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Ident;
    static deserializeBinaryFromReader(message: Ident, reader: jspb.BinaryReader): Ident;
  }

  export namespace Ident {
    export type AsObject = {
      name: string,
    }
  }

  export class Select extends jspb.Message {
    hasOperand(): boolean;
    clearOperand(): void;
    getOperand(): Expr | undefined;
    setOperand(value?: Expr): void;

    getField(): string;
    setField(value: string): void;

    getTestOnly(): boolean;
    setTestOnly(value: boolean): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Select.AsObject;
    static toObject(includeInstance: boolean, msg: Select): Select.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Select, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Select;
    static deserializeBinaryFromReader(message: Select, reader: jspb.BinaryReader): Select;
  }

  export namespace Select {
    export type AsObject = {
      operand?: Expr.AsObject,
      field: string,
      testOnly: boolean,
    }
  }

  export class Call extends jspb.Message {
    hasTarget(): boolean;
    clearTarget(): void;
    getTarget(): Expr | undefined;
    setTarget(value?: Expr): void;

    getFunction(): string;
    setFunction(value: string): void;

    clearArgsList(): void;
    getArgsList(): Array<Expr>;
    setArgsList(value: Array<Expr>): void;
    addArgs(value?: Expr, index?: number): Expr;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Call.AsObject;
    static toObject(includeInstance: boolean, msg: Call): Call.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Call, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Call;
    static deserializeBinaryFromReader(message: Call, reader: jspb.BinaryReader): Call;
  }

  export namespace Call {
    export type AsObject = {
      target?: Expr.AsObject,
      pb_function: string,
      argsList: Array<Expr.AsObject>,
    }
  }

  export class CreateList extends jspb.Message {
    clearElementsList(): void;
    getElementsList(): Array<Expr>;
    setElementsList(value: Array<Expr>): void;
    addElements(value?: Expr, index?: number): Expr;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateList.AsObject;
    static toObject(includeInstance: boolean, msg: CreateList): CreateList.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateList, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateList;
    static deserializeBinaryFromReader(message: CreateList, reader: jspb.BinaryReader): CreateList;
  }

  export namespace CreateList {
    export type AsObject = {
      elementsList: Array<Expr.AsObject>,
    }
  }

  export class CreateStruct extends jspb.Message {
    getType(): string;
    setType(value: string): void;

    clearEntriesList(): void;
    getEntriesList(): Array<Expr.CreateStruct.Entry>;
    setEntriesList(value: Array<Expr.CreateStruct.Entry>): void;
    addEntries(value?: Expr.CreateStruct.Entry, index?: number): Expr.CreateStruct.Entry;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): CreateStruct.AsObject;
    static toObject(includeInstance: boolean, msg: CreateStruct): CreateStruct.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: CreateStruct, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): CreateStruct;
    static deserializeBinaryFromReader(message: CreateStruct, reader: jspb.BinaryReader): CreateStruct;
  }

  export namespace CreateStruct {
    export type AsObject = {
      type: string,
      entriesList: Array<Expr.CreateStruct.Entry.AsObject>,
    }

    export class Entry extends jspb.Message {
      getId(): number;
      setId(value: number): void;

      hasFieldKey(): boolean;
      clearFieldKey(): void;
      getFieldKey(): string;
      setFieldKey(value: string): void;

      hasMapKey(): boolean;
      clearMapKey(): void;
      getMapKey(): Expr | undefined;
      setMapKey(value?: Expr): void;

      hasValue(): boolean;
      clearValue(): void;
      getValue(): Expr | undefined;
      setValue(value?: Expr): void;

      getKeyKindCase(): Entry.KeyKindCase;
      serializeBinary(): Uint8Array;
      toObject(includeInstance?: boolean): Entry.AsObject;
      static toObject(includeInstance: boolean, msg: Entry): Entry.AsObject;
      static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
      static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
      static serializeBinaryToWriter(message: Entry, writer: jspb.BinaryWriter): void;
      static deserializeBinary(bytes: Uint8Array): Entry;
      static deserializeBinaryFromReader(message: Entry, reader: jspb.BinaryReader): Entry;
    }

    export namespace Entry {
      export type AsObject = {
        id: number,
        fieldKey: string,
        mapKey?: Expr.AsObject,
        value?: Expr.AsObject,
      }

      export enum KeyKindCase {
        KEY_KIND_NOT_SET = 0,
        FIELD_KEY = 2,
        MAP_KEY = 3,
      }
    }
  }

  export class Comprehension extends jspb.Message {
    getIterVar(): string;
    setIterVar(value: string): void;

    hasIterRange(): boolean;
    clearIterRange(): void;
    getIterRange(): Expr | undefined;
    setIterRange(value?: Expr): void;

    getAccuVar(): string;
    setAccuVar(value: string): void;

    hasAccuInit(): boolean;
    clearAccuInit(): void;
    getAccuInit(): Expr | undefined;
    setAccuInit(value?: Expr): void;

    hasLoopCondition(): boolean;
    clearLoopCondition(): void;
    getLoopCondition(): Expr | undefined;
    setLoopCondition(value?: Expr): void;

    hasLoopStep(): boolean;
    clearLoopStep(): void;
    getLoopStep(): Expr | undefined;
    setLoopStep(value?: Expr): void;

    hasResult(): boolean;
    clearResult(): void;
    getResult(): Expr | undefined;
    setResult(value?: Expr): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): Comprehension.AsObject;
    static toObject(includeInstance: boolean, msg: Comprehension): Comprehension.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: Comprehension, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): Comprehension;
    static deserializeBinaryFromReader(message: Comprehension, reader: jspb.BinaryReader): Comprehension;
  }

  export namespace Comprehension {
    export type AsObject = {
      iterVar: string,
      iterRange?: Expr.AsObject,
      accuVar: string,
      accuInit?: Expr.AsObject,
      loopCondition?: Expr.AsObject,
      loopStep?: Expr.AsObject,
      result?: Expr.AsObject,
    }
  }

  export enum ExprKindCase {
    EXPR_KIND_NOT_SET = 0,
    LITERAL_EXPR = 3,
    IDENT_EXPR = 4,
    SELECT_EXPR = 5,
    CALL_EXPR = 6,
    LIST_EXPR = 7,
    STRUCT_EXPR = 8,
    COMPREHENSION_EXPR = 9,
  }
}

export class Literal extends jspb.Message {
  hasNullValue(): boolean;
  clearNullValue(): void;
  getNullValue(): google_protobuf_struct_pb.NullValueMap[keyof google_protobuf_struct_pb.NullValueMap];
  setNullValue(value: google_protobuf_struct_pb.NullValueMap[keyof google_protobuf_struct_pb.NullValueMap]): void;

  hasBoolValue(): boolean;
  clearBoolValue(): void;
  getBoolValue(): boolean;
  setBoolValue(value: boolean): void;

  hasInt64Value(): boolean;
  clearInt64Value(): void;
  getInt64Value(): number;
  setInt64Value(value: number): void;

  hasUint64Value(): boolean;
  clearUint64Value(): void;
  getUint64Value(): number;
  setUint64Value(value: number): void;

  hasDoubleValue(): boolean;
  clearDoubleValue(): void;
  getDoubleValue(): number;
  setDoubleValue(value: number): void;

  hasStringValue(): boolean;
  clearStringValue(): void;
  getStringValue(): string;
  setStringValue(value: string): void;

  hasBytesValue(): boolean;
  clearBytesValue(): void;
  getBytesValue(): Uint8Array | string;
  getBytesValue_asU8(): Uint8Array;
  getBytesValue_asB64(): string;
  setBytesValue(value: Uint8Array | string): void;

  getConstantKindCase(): Literal.ConstantKindCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Literal.AsObject;
  static toObject(includeInstance: boolean, msg: Literal): Literal.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Literal, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Literal;
  static deserializeBinaryFromReader(message: Literal, reader: jspb.BinaryReader): Literal;
}

export namespace Literal {
  export type AsObject = {
    nullValue: google_protobuf_struct_pb.NullValueMap[keyof google_protobuf_struct_pb.NullValueMap],
    boolValue: boolean,
    int64Value: number,
    uint64Value: number,
    doubleValue: number,
    stringValue: string,
    bytesValue: Uint8Array | string,
  }

  export enum ConstantKindCase {
    CONSTANT_KIND_NOT_SET = 0,
    NULL_VALUE = 1,
    BOOL_VALUE = 2,
    INT64_VALUE = 3,
    UINT64_VALUE = 4,
    DOUBLE_VALUE = 5,
    STRING_VALUE = 6,
    BYTES_VALUE = 7,
  }
}

