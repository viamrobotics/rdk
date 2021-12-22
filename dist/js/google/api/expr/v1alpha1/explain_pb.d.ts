// package: google.api.expr.v1alpha1
// file: google/api/expr/v1alpha1/explain.proto

import * as jspb from "google-protobuf";
import * as google_api_expr_v1alpha1_value_pb from "../../../../google/api/expr/v1alpha1/value_pb";

export class Explain extends jspb.Message {
  clearValuesList(): void;
  getValuesList(): Array<google_api_expr_v1alpha1_value_pb.Value>;
  setValuesList(value: Array<google_api_expr_v1alpha1_value_pb.Value>): void;
  addValues(value?: google_api_expr_v1alpha1_value_pb.Value, index?: number): google_api_expr_v1alpha1_value_pb.Value;

  clearExprStepsList(): void;
  getExprStepsList(): Array<Explain.ExprStep>;
  setExprStepsList(value: Array<Explain.ExprStep>): void;
  addExprSteps(value?: Explain.ExprStep, index?: number): Explain.ExprStep;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): Explain.AsObject;
  static toObject(includeInstance: boolean, msg: Explain): Explain.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: Explain, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): Explain;
  static deserializeBinaryFromReader(message: Explain, reader: jspb.BinaryReader): Explain;
}

export namespace Explain {
  export type AsObject = {
    valuesList: Array<google_api_expr_v1alpha1_value_pb.Value.AsObject>,
    exprStepsList: Array<Explain.ExprStep.AsObject>,
  }

  export class ExprStep extends jspb.Message {
    getId(): number;
    setId(value: number): void;

    getValueIndex(): number;
    setValueIndex(value: number): void;

    serializeBinary(): Uint8Array;
    toObject(includeInstance?: boolean): ExprStep.AsObject;
    static toObject(includeInstance: boolean, msg: ExprStep): ExprStep.AsObject;
    static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
    static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
    static serializeBinaryToWriter(message: ExprStep, writer: jspb.BinaryWriter): void;
    static deserializeBinary(bytes: Uint8Array): ExprStep;
    static deserializeBinaryFromReader(message: ExprStep, reader: jspb.BinaryReader): ExprStep;
  }

  export namespace ExprStep {
    export type AsObject = {
      id: number,
      valueIndex: number,
    }
  }
}

