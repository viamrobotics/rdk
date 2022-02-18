// package: google.api.expr.v1alpha1
// file: google/api/expr/v1alpha1/checked.proto

import * as jspb from "google-protobuf";
import * as google_api_expr_v1alpha1_syntax_pb from "../../../../google/api/expr/v1alpha1/syntax_pb";
import * as google_protobuf_empty_pb from "google-protobuf/google/protobuf/empty_pb";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";

export class CheckedExpr extends jspb.Message {
  getReferenceMapMap(): jspb.Map<number, Reference>;
  clearReferenceMapMap(): void;
  getTypeMapMap(): jspb.Map<number, Type>;
  clearTypeMapMap(): void;
  hasSourceInfo(): boolean;
  clearSourceInfo(): void;
  getSourceInfo(): google_api_expr_v1alpha1_syntax_pb.SourceInfo | undefined;
  setSourceInfo(value?: google_api_expr_v1alpha1_syntax_pb.SourceInfo): void;

  getExprVersion(): string;
  setExprVersion(value: string): void;

  hasExpr(): boolean;
  clearExpr(): void;
  getExpr(): google_api_expr_v1alpha1_syntax_pb.Expr | undefined;
  setExpr(value?: google_api_expr_v1alpha1_syntax_pb.Expr): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): CheckedExpr.AsObject;
  static toObject(includeInstance: boolean, msg: CheckedExpr): CheckedExpr.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: CheckedExpr, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): CheckedExpr;
  static deserializeBinaryFromReader(message: CheckedExpr, reader: jspb.BinaryReader): CheckedExpr;
}

export namespace CheckedExpr {
  export type AsObject = {
    referenceMapMap: Array<[number, Reference.AsObject]>,
    typeMapMap: Array<[number, Type.AsObject]>,
    sourceInfo?: google_api_expr_v1alpha1_syntax_pb.SourceInfo.AsObject,
    exprVersion: string,
    expr?: google_api_expr_v1alpha1_syntax_pb.Expr.AsObject,
  }
}

export class Type extends jspb.Message {
  hasDyn(): boolean;
  clearDyn(): void;
  getDyn(): google_protobuf_empty_pb.Empty | undefined;
  setDyn(value?: google_protobuf_empty_pb.Empty): void;

  hasNull(): boolean;
  clearNull(): void;
  getNull(): google_protobuf_struct_pb.NullValueMap[keyof google_protobuf_struct_pb.NullValueMap];
  setNull(value: google_protobuf_struct_pb.NullValueMap[keyof google_protobuf_struct_pb.NullValueMap]): void;

  hasPrimitive(): boolean;
  clearPrimitive(): void;
  getPrimitive(): Type.PrimitiveTypeMap[keyof Type.PrimitiveTypeMap];
  setPrimitive(value: Type.PrimitiveTypeMap[keyof Type.PrimitiveTypeMap]): void;

  hasWrapper(): boolean;
  clearWrapper(): void;
  getWrapper(): Type.PrimitiveTypeMap[keyof Type.PrimitiveTypeMap];
  setWrapper(value: Type.PrimitiveTypeMap[keyof Type.PrimitiveTypeMap]): void;

  hasWellKnown(): boolean;
  clearWellKnown(): void;
  getWellKnown(): Type.WellKnownTypeMap[keyof Type.WellKnownTypeMap];
  setWellKnown(value: Type.WellKnownTypeMap[keyof Type.WellKnownTypeMap]): void;

  hasListType(): boolean;
  clearListType(): void;
  getListType(): Type.ListType | undefined;
  setListType(value?: Type.ListType): void;

  hasMapType(): boolean;
  clearMapType(): void;
  getMapType(): Type.MapType | undefined;
  setMapType(value?: Type.MapType): void;

  hasFunction(): boolean;
  clearFunction(): void;
  getFunction(): Type.FunctionType | undefined;
  setFunction(value?: Type.FunctionType): void;

  hasMessageType(): boolean;
  clearMessageType(): void;
  getMessageType(): string;
  setMessageType(value: string): void;

  hasTypeParam(): boolean;
  clearTypeParam(): void;
  getTypeParam(): string;
  setTypeParam(value: string): void;

  hasType(): boolean;
  clearType(): void;
  getType(): Type | undefined;
  setType(value?: Type): void;

  hasError(): boolean;
  clearError(): void;
  getError(): google_protobuf_empty_pb.Empty | undefined;
  setError(value?: google_protobuf_empty_pb.Empty): void;

  hasAbstractType(): boolean;
  clearAbstractType(): void;
  getAbstractType(): Type.AbstractType | undefined;
  setAbstractType(value?: Type.AbstractType): void;

  getTypeKindCase(): Type.TypeKindCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Type.AsObject;
  static toObject(includeInstance: boolean, msg: Type): Type.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Type, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Type;
  static deserializeBinaryFromReader(message: Type, reader: jspb.BinaryReader): Type;
}

export namespace Type {
  export type AsObject = {
    dyn?: google_protobuf_empty_pb.Empty.AsObject,
    pb_null: google_protobuf_struct_pb.NullValueMap[keyof google_protobuf_struct_pb.NullValueMap],
    primitive: Type.PrimitiveTypeMap[keyof Type.PrimitiveTypeMap],
    wrapper: Type.PrimitiveTypeMap[keyof Type.PrimitiveTypeMap],
    wellKnown: Type.WellKnownTypeMap[keyof Type.WellKnownTypeMap],
    listType?: Type.ListType.AsObject,
    mapType?: Type.MapType.AsObject,
    pb_function?: Type.FunctionType.AsObject,
    messageType: string,
    typeParam: string,
    type?: Type.AsObject,
    error?: google_protobuf_empty_pb.Empty.AsObject,
    abstractType?: Type.AbstractType.AsObject,
  }

  export class ListType extends jspb.Message {
    hasElemType(): boolean;
    clearElemType(): void;
    getElemType(): Type | undefined;
    setElemType(value?: Type): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ListType.AsObject;
    static toObject(includeInstance: boolean, msg: ListType): ListType.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ListType, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ListType;
    static deserializeBinaryFromReader(message: ListType, reader: jspb.BinaryReader): ListType;
  }

  export namespace ListType {
    export type AsObject = {
      elemType?: Type.AsObject,
    }
  }

  export class MapType extends jspb.Message {
    hasKeyType(): boolean;
    clearKeyType(): void;
    getKeyType(): Type | undefined;
    setKeyType(value?: Type): void;

    hasValueType(): boolean;
    clearValueType(): void;
    getValueType(): Type | undefined;
    setValueType(value?: Type): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): MapType.AsObject;
    static toObject(includeInstance: boolean, msg: MapType): MapType.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: MapType, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): MapType;
    static deserializeBinaryFromReader(message: MapType, reader: jspb.BinaryReader): MapType;
  }

  export namespace MapType {
    export type AsObject = {
      keyType?: Type.AsObject,
      valueType?: Type.AsObject,
    }
  }

  export class FunctionType extends jspb.Message {
    hasResultType(): boolean;
    clearResultType(): void;
    getResultType(): Type | undefined;
    setResultType(value?: Type): void;

    clearArgTypesList(): void;
    getArgTypesList(): Array<Type>;
    setArgTypesList(value: Array<Type>): void;
    addArgTypes(value?: Type, index?: number): Type;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FunctionType.AsObject;
    static toObject(includeInstance: boolean, msg: FunctionType): FunctionType.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FunctionType, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FunctionType;
    static deserializeBinaryFromReader(message: FunctionType, reader: jspb.BinaryReader): FunctionType;
  }

  export namespace FunctionType {
    export type AsObject = {
      resultType?: Type.AsObject,
      argTypesList: Array<Type.AsObject>,
    }
  }

  export class AbstractType extends jspb.Message {
    getName(): string;
    setName(value: string): void;

    clearParameterTypesList(): void;
    getParameterTypesList(): Array<Type>;
    setParameterTypesList(value: Array<Type>): void;
    addParameterTypes(value?: Type, index?: number): Type;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): AbstractType.AsObject;
    static toObject(includeInstance: boolean, msg: AbstractType): AbstractType.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: AbstractType, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): AbstractType;
    static deserializeBinaryFromReader(message: AbstractType, reader: jspb.BinaryReader): AbstractType;
  }

  export namespace AbstractType {
    export type AsObject = {
      name: string,
      parameterTypesList: Array<Type.AsObject>,
    }
  }

  export interface PrimitiveTypeMap {
    PRIMITIVE_TYPE_UNSPECIFIED: 0;
    BOOL: 1;
    INT64: 2;
    UINT64: 3;
    DOUBLE: 4;
    STRING: 5;
    BYTES: 6;
  }

  export const PrimitiveType: PrimitiveTypeMap;

  export interface WellKnownTypeMap {
    WELL_KNOWN_TYPE_UNSPECIFIED: 0;
    ANY: 1;
    TIMESTAMP: 2;
    DURATION: 3;
  }

  export const WellKnownType: WellKnownTypeMap;

  export enum TypeKindCase {
    TYPE_KIND_NOT_SET = 0,
    DYN = 1,
    NULL = 2,
    PRIMITIVE = 3,
    WRAPPER = 4,
    WELL_KNOWN = 5,
    LIST_TYPE = 6,
    MAP_TYPE = 7,
    FUNCTION = 8,
    MESSAGE_TYPE = 9,
    TYPE_PARAM = 10,
    TYPE = 11,
    ERROR = 12,
    ABSTRACT_TYPE = 14,
  }
}

export class Decl extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasIdent(): boolean;
  clearIdent(): void;
  getIdent(): Decl.IdentDecl | undefined;
  setIdent(value?: Decl.IdentDecl): void;

  hasFunction(): boolean;
  clearFunction(): void;
  getFunction(): Decl.FunctionDecl | undefined;
  setFunction(value?: Decl.FunctionDecl): void;

  getDeclKindCase(): Decl.DeclKindCase;
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Decl.AsObject;
  static toObject(includeInstance: boolean, msg: Decl): Decl.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Decl, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Decl;
  static deserializeBinaryFromReader(message: Decl, reader: jspb.BinaryReader): Decl;
}

export namespace Decl {
  export type AsObject = {
    name: string,
    ident?: Decl.IdentDecl.AsObject,
    pb_function?: Decl.FunctionDecl.AsObject,
  }

  export class IdentDecl extends jspb.Message {
    hasType(): boolean;
    clearType(): void;
    getType(): Type | undefined;
    setType(value?: Type): void;

    hasValue(): boolean;
    clearValue(): void;
    getValue(): google_api_expr_v1alpha1_syntax_pb.Constant | undefined;
    setValue(value?: google_api_expr_v1alpha1_syntax_pb.Constant): void;

    getDoc(): string;
    setDoc(value: string): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): IdentDecl.AsObject;
    static toObject(includeInstance: boolean, msg: IdentDecl): IdentDecl.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: IdentDecl, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): IdentDecl;
    static deserializeBinaryFromReader(message: IdentDecl, reader: jspb.BinaryReader): IdentDecl;
  }

  export namespace IdentDecl {
    export type AsObject = {
      type?: Type.AsObject,
      value?: google_api_expr_v1alpha1_syntax_pb.Constant.AsObject,
      doc: string,
    }
  }

  export class FunctionDecl extends jspb.Message {
    clearOverloadsList(): void;
    getOverloadsList(): Array<Decl.FunctionDecl.Overload>;
    setOverloadsList(value: Array<Decl.FunctionDecl.Overload>): void;
    addOverloads(value?: Decl.FunctionDecl.Overload, index?: number): Decl.FunctionDecl.Overload;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): FunctionDecl.AsObject;
    static toObject(includeInstance: boolean, msg: FunctionDecl): FunctionDecl.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: FunctionDecl, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): FunctionDecl;
    static deserializeBinaryFromReader(message: FunctionDecl, reader: jspb.BinaryReader): FunctionDecl;
  }

  export namespace FunctionDecl {
    export type AsObject = {
      overloadsList: Array<Decl.FunctionDecl.Overload.AsObject>,
    }

    export class Overload extends jspb.Message {
      getOverloadId(): string;
      setOverloadId(value: string): void;

      clearParamsList(): void;
      getParamsList(): Array<Type>;
      setParamsList(value: Array<Type>): void;
      addParams(value?: Type, index?: number): Type;

      clearTypeParamsList(): void;
      getTypeParamsList(): Array<string>;
      setTypeParamsList(value: Array<string>): void;
      addTypeParams(value: string, index?: number): string;

      hasResultType(): boolean;
      clearResultType(): void;
      getResultType(): Type | undefined;
      setResultType(value?: Type): void;

      getIsInstanceFunction(): boolean;
      setIsInstanceFunction(value: boolean): void;

      getDoc(): string;
      setDoc(value: string): void;

      serializeBinary(): Uint8Array;
      toObject(includeInstance?: boolean): Overload.AsObject;
      static toObject(includeInstance: boolean, msg: Overload): Overload.AsObject;
      static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
      static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
      static serializeBinaryToWriter(message: Overload, writer: jspb.BinaryWriter): void;
      static deserializeBinary(bytes: Uint8Array): Overload;
      static deserializeBinaryFromReader(message: Overload, reader: jspb.BinaryReader): Overload;
    }

    export namespace Overload {
      export type AsObject = {
        overloadId: string,
        paramsList: Array<Type.AsObject>,
        typeParamsList: Array<string>,
        resultType?: Type.AsObject,
        isInstanceFunction: boolean,
        doc: string,
      }
    }
  }

  export enum DeclKindCase {
    DECL_KIND_NOT_SET = 0,
    IDENT = 2,
    FUNCTION = 3,
  }
}

export class Reference extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  clearOverloadIdList(): void;
  getOverloadIdList(): Array<string>;
  setOverloadIdList(value: Array<string>): void;
  addOverloadId(value: string, index?: number): string;

  hasValue(): boolean;
  clearValue(): void;
  getValue(): google_api_expr_v1alpha1_syntax_pb.Constant | undefined;
  setValue(value?: google_api_expr_v1alpha1_syntax_pb.Constant): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Reference.AsObject;
  static toObject(includeInstance: boolean, msg: Reference): Reference.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Reference, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Reference;
  static deserializeBinaryFromReader(message: Reference, reader: jspb.BinaryReader): Reference;
}

export namespace Reference {
  export type AsObject = {
    name: string,
    overloadIdList: Array<string>,
    value?: google_api_expr_v1alpha1_syntax_pb.Constant.AsObject,
  }
}

