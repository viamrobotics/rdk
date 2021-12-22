// package: proto.api.component.v1
// file: proto/api/component/v1/motor.proto

import * as jspb from "google-protobuf";
import * as google_protobuf_struct_pb from "google-protobuf/google/protobuf/struct_pb";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";

export class MotorServiceGetPIDConfigRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGetPIDConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGetPIDConfigRequest): MotorServiceGetPIDConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGetPIDConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGetPIDConfigRequest;
  static deserializeBinaryFromReader(message: MotorServiceGetPIDConfigRequest, reader: jspb.BinaryReader): MotorServiceGetPIDConfigRequest;
}

export namespace MotorServiceGetPIDConfigRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorServiceGetPIDConfigResponse extends jspb.Message {
  hasPidConfig(): boolean;
  clearPidConfig(): void;
  getPidConfig(): google_protobuf_struct_pb.Struct | undefined;
  setPidConfig(value?: google_protobuf_struct_pb.Struct): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGetPIDConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGetPIDConfigResponse): MotorServiceGetPIDConfigResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGetPIDConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGetPIDConfigResponse;
  static deserializeBinaryFromReader(message: MotorServiceGetPIDConfigResponse, reader: jspb.BinaryReader): MotorServiceGetPIDConfigResponse;
}

export namespace MotorServiceGetPIDConfigResponse {
  export type AsObject = {
    pidConfig?: google_protobuf_struct_pb.Struct.AsObject,
  }
}

export class MotorServiceSetPIDConfigRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  hasPidConfig(): boolean;
  clearPidConfig(): void;
  getPidConfig(): google_protobuf_struct_pb.Struct | undefined;
  setPidConfig(value?: google_protobuf_struct_pb.Struct): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceSetPIDConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceSetPIDConfigRequest): MotorServiceSetPIDConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceSetPIDConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceSetPIDConfigRequest;
  static deserializeBinaryFromReader(message: MotorServiceSetPIDConfigRequest, reader: jspb.BinaryReader): MotorServiceSetPIDConfigRequest;
}

export namespace MotorServiceSetPIDConfigRequest {
  export type AsObject = {
    name: string,
    pidConfig?: google_protobuf_struct_pb.Struct.AsObject,
  }
}

export class MotorServiceSetPIDConfigResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceSetPIDConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceSetPIDConfigResponse): MotorServiceSetPIDConfigResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceSetPIDConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceSetPIDConfigResponse;
  static deserializeBinaryFromReader(message: MotorServiceSetPIDConfigResponse, reader: jspb.BinaryReader): MotorServiceSetPIDConfigResponse;
}

export namespace MotorServiceSetPIDConfigResponse {
  export type AsObject = {
  }
}

export class MotorServicePIDStepRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getSetPoint(): number;
  setSetPoint(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServicePIDStepRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServicePIDStepRequest): MotorServicePIDStepRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServicePIDStepRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServicePIDStepRequest;
  static deserializeBinaryFromReader(message: MotorServicePIDStepRequest, reader: jspb.BinaryReader): MotorServicePIDStepRequest;
}

export namespace MotorServicePIDStepRequest {
  export type AsObject = {
    name: string,
    setPoint: number,
  }
}

export class MotorServicePIDStepResponse extends jspb.Message {
  getTime(): number;
  setTime(value: number): void;

  getSetPoint(): number;
  setSetPoint(value: number): void;

  getRefValue(): number;
  setRefValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServicePIDStepResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServicePIDStepResponse): MotorServicePIDStepResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServicePIDStepResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServicePIDStepResponse;
  static deserializeBinaryFromReader(message: MotorServicePIDStepResponse, reader: jspb.BinaryReader): MotorServicePIDStepResponse;
}

export namespace MotorServicePIDStepResponse {
  export type AsObject = {
    time: number,
    setPoint: number,
    refValue: number,
  }
}

export class MotorServiceSetPowerRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPowerPct(): number;
  setPowerPct(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceSetPowerRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceSetPowerRequest): MotorServiceSetPowerRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceSetPowerRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceSetPowerRequest;
  static deserializeBinaryFromReader(message: MotorServiceSetPowerRequest, reader: jspb.BinaryReader): MotorServiceSetPowerRequest;
}

export namespace MotorServiceSetPowerRequest {
  export type AsObject = {
    name: string,
    powerPct: number,
  }
}

export class MotorServiceSetPowerResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceSetPowerResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceSetPowerResponse): MotorServiceSetPowerResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceSetPowerResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceSetPowerResponse;
  static deserializeBinaryFromReader(message: MotorServiceSetPowerResponse, reader: jspb.BinaryReader): MotorServiceSetPowerResponse;
}

export namespace MotorServiceSetPowerResponse {
  export type AsObject = {
  }
}

export class MotorServiceGoRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPowerPct(): number;
  setPowerPct(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoRequest): MotorServiceGoRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoRequest;
  static deserializeBinaryFromReader(message: MotorServiceGoRequest, reader: jspb.BinaryReader): MotorServiceGoRequest;
}

export namespace MotorServiceGoRequest {
  export type AsObject = {
    name: string,
    powerPct: number,
  }
}

export class MotorServiceGoResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoResponse): MotorServiceGoResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoResponse;
  static deserializeBinaryFromReader(message: MotorServiceGoResponse, reader: jspb.BinaryReader): MotorServiceGoResponse;
}

export namespace MotorServiceGoResponse {
  export type AsObject = {
  }
}

export class MotorServiceGoForRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getRpm(): number;
  setRpm(value: number): void;

  getRevolutions(): number;
  setRevolutions(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoForRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoForRequest): MotorServiceGoForRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoForRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoForRequest;
  static deserializeBinaryFromReader(message: MotorServiceGoForRequest, reader: jspb.BinaryReader): MotorServiceGoForRequest;
}

export namespace MotorServiceGoForRequest {
  export type AsObject = {
    name: string,
    rpm: number,
    revolutions: number,
  }
}

export class MotorServiceGoForResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoForResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoForResponse): MotorServiceGoForResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoForResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoForResponse;
  static deserializeBinaryFromReader(message: MotorServiceGoForResponse, reader: jspb.BinaryReader): MotorServiceGoForResponse;
}

export namespace MotorServiceGoForResponse {
  export type AsObject = {
  }
}

export class MotorServiceGoToRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getRpm(): number;
  setRpm(value: number): void;

  getPosition(): number;
  setPosition(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoToRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoToRequest): MotorServiceGoToRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoToRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoToRequest;
  static deserializeBinaryFromReader(message: MotorServiceGoToRequest, reader: jspb.BinaryReader): MotorServiceGoToRequest;
}

export namespace MotorServiceGoToRequest {
  export type AsObject = {
    name: string,
    rpm: number,
    position: number,
  }
}

export class MotorServiceGoToResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoToResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoToResponse): MotorServiceGoToResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoToResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoToResponse;
  static deserializeBinaryFromReader(message: MotorServiceGoToResponse, reader: jspb.BinaryReader): MotorServiceGoToResponse;
}

export namespace MotorServiceGoToResponse {
  export type AsObject = {
  }
}

export class MotorServiceGoTillStopRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getRpm(): number;
  setRpm(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoTillStopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoTillStopRequest): MotorServiceGoTillStopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoTillStopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoTillStopRequest;
  static deserializeBinaryFromReader(message: MotorServiceGoTillStopRequest, reader: jspb.BinaryReader): MotorServiceGoTillStopRequest;
}

export namespace MotorServiceGoTillStopRequest {
  export type AsObject = {
    name: string,
    rpm: number,
  }
}

export class MotorServiceGoTillStopResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceGoTillStopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceGoTillStopResponse): MotorServiceGoTillStopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceGoTillStopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceGoTillStopResponse;
  static deserializeBinaryFromReader(message: MotorServiceGoTillStopResponse, reader: jspb.BinaryReader): MotorServiceGoTillStopResponse;
}

export namespace MotorServiceGoTillStopResponse {
  export type AsObject = {
  }
}

export class MotorServiceResetZeroPositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getOffset(): number;
  setOffset(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceResetZeroPositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceResetZeroPositionRequest): MotorServiceResetZeroPositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceResetZeroPositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceResetZeroPositionRequest;
  static deserializeBinaryFromReader(message: MotorServiceResetZeroPositionRequest, reader: jspb.BinaryReader): MotorServiceResetZeroPositionRequest;
}

export namespace MotorServiceResetZeroPositionRequest {
  export type AsObject = {
    name: string,
    offset: number,
  }
}

export class MotorServiceResetZeroPositionResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceResetZeroPositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceResetZeroPositionResponse): MotorServiceResetZeroPositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceResetZeroPositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceResetZeroPositionResponse;
  static deserializeBinaryFromReader(message: MotorServiceResetZeroPositionResponse, reader: jspb.BinaryReader): MotorServiceResetZeroPositionResponse;
}

export namespace MotorServiceResetZeroPositionResponse {
  export type AsObject = {
  }
}

export class MotorServicePositionRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServicePositionRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServicePositionRequest): MotorServicePositionRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServicePositionRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServicePositionRequest;
  static deserializeBinaryFromReader(message: MotorServicePositionRequest, reader: jspb.BinaryReader): MotorServicePositionRequest;
}

export namespace MotorServicePositionRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorServicePositionResponse extends jspb.Message {
  getPosition(): number;
  setPosition(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServicePositionResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServicePositionResponse): MotorServicePositionResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServicePositionResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServicePositionResponse;
  static deserializeBinaryFromReader(message: MotorServicePositionResponse, reader: jspb.BinaryReader): MotorServicePositionResponse;
}

export namespace MotorServicePositionResponse {
  export type AsObject = {
    position: number,
  }
}

export class MotorServicePositionSupportedRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServicePositionSupportedRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServicePositionSupportedRequest): MotorServicePositionSupportedRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServicePositionSupportedRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServicePositionSupportedRequest;
  static deserializeBinaryFromReader(message: MotorServicePositionSupportedRequest, reader: jspb.BinaryReader): MotorServicePositionSupportedRequest;
}

export namespace MotorServicePositionSupportedRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorServicePositionSupportedResponse extends jspb.Message {
  getSupported(): boolean;
  setSupported(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServicePositionSupportedResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServicePositionSupportedResponse): MotorServicePositionSupportedResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServicePositionSupportedResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServicePositionSupportedResponse;
  static deserializeBinaryFromReader(message: MotorServicePositionSupportedResponse, reader: jspb.BinaryReader): MotorServicePositionSupportedResponse;
}

export namespace MotorServicePositionSupportedResponse {
  export type AsObject = {
    supported: boolean,
  }
}

export class MotorServiceStopRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceStopRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceStopRequest): MotorServiceStopRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceStopRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceStopRequest;
  static deserializeBinaryFromReader(message: MotorServiceStopRequest, reader: jspb.BinaryReader): MotorServiceStopRequest;
}

export namespace MotorServiceStopRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorServiceStopResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceStopResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceStopResponse): MotorServiceStopResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceStopResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceStopResponse;
  static deserializeBinaryFromReader(message: MotorServiceStopResponse, reader: jspb.BinaryReader): MotorServiceStopResponse;
}

export namespace MotorServiceStopResponse {
  export type AsObject = {
  }
}

export class MotorServiceIsOnRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceIsOnRequest.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceIsOnRequest): MotorServiceIsOnRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceIsOnRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceIsOnRequest;
  static deserializeBinaryFromReader(message: MotorServiceIsOnRequest, reader: jspb.BinaryReader): MotorServiceIsOnRequest;
}

export namespace MotorServiceIsOnRequest {
  export type AsObject = {
    name: string,
  }
}

export class MotorServiceIsOnResponse extends jspb.Message {
  getIsOn(): boolean;
  setIsOn(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): MotorServiceIsOnResponse.AsObject;
  static toObject(includeInstance: boolean, msg: MotorServiceIsOnResponse): MotorServiceIsOnResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: MotorServiceIsOnResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): MotorServiceIsOnResponse;
  static deserializeBinaryFromReader(message: MotorServiceIsOnResponse, reader: jspb.BinaryReader): MotorServiceIsOnResponse;
}

export namespace MotorServiceIsOnResponse {
  export type AsObject = {
    isOn: boolean,
  }
}

