// package: proto.api.component.v1
// file: proto/api/component/v1/board.proto

import * as jspb from "google-protobuf";
import * as google_api_annotations_pb from "../../../../google/api/annotations_pb";
import * as proto_api_common_v1_common_pb from "../../../../proto/api/common/v1/common_pb";

export class BoardServiceStatusRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceStatusRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceStatusRequest): BoardServiceStatusRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceStatusRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceStatusRequest;
  static deserializeBinaryFromReader(message: BoardServiceStatusRequest, reader: jspb.BinaryReader): BoardServiceStatusRequest;
}

export namespace BoardServiceStatusRequest {
  export type AsObject = {
    name: string,
  }
}

export class BoardServiceStatusResponse extends jspb.Message {
  hasStatus(): boolean;
  clearStatus(): void;
  getStatus(): proto_api_common_v1_common_pb.BoardStatus | undefined;
  setStatus(value?: proto_api_common_v1_common_pb.BoardStatus): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceStatusResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceStatusResponse): BoardServiceStatusResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceStatusResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceStatusResponse;
  static deserializeBinaryFromReader(message: BoardServiceStatusResponse, reader: jspb.BinaryReader): BoardServiceStatusResponse;
}

export namespace BoardServiceStatusResponse {
  export type AsObject = {
    status?: proto_api_common_v1_common_pb.BoardStatus.AsObject,
  }
}

export class BoardServiceGPIOSetRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getHigh(): boolean;
  setHigh(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGPIOSetRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGPIOSetRequest): BoardServiceGPIOSetRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGPIOSetRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGPIOSetRequest;
  static deserializeBinaryFromReader(message: BoardServiceGPIOSetRequest, reader: jspb.BinaryReader): BoardServiceGPIOSetRequest;
}

export namespace BoardServiceGPIOSetRequest {
  export type AsObject = {
    name: string,
    pin: string,
    high: boolean,
  }
}

export class BoardServiceGPIOSetResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGPIOSetResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGPIOSetResponse): BoardServiceGPIOSetResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGPIOSetResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGPIOSetResponse;
  static deserializeBinaryFromReader(message: BoardServiceGPIOSetResponse, reader: jspb.BinaryReader): BoardServiceGPIOSetResponse;
}

export namespace BoardServiceGPIOSetResponse {
  export type AsObject = {
  }
}

export class BoardServiceGPIOGetRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGPIOGetRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGPIOGetRequest): BoardServiceGPIOGetRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGPIOGetRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGPIOGetRequest;
  static deserializeBinaryFromReader(message: BoardServiceGPIOGetRequest, reader: jspb.BinaryReader): BoardServiceGPIOGetRequest;
}

export namespace BoardServiceGPIOGetRequest {
  export type AsObject = {
    name: string,
    pin: string,
  }
}

export class BoardServiceGPIOGetResponse extends jspb.Message {
  getHigh(): boolean;
  setHigh(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGPIOGetResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGPIOGetResponse): BoardServiceGPIOGetResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGPIOGetResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGPIOGetResponse;
  static deserializeBinaryFromReader(message: BoardServiceGPIOGetResponse, reader: jspb.BinaryReader): BoardServiceGPIOGetResponse;
}

export namespace BoardServiceGPIOGetResponse {
  export type AsObject = {
    high: boolean,
  }
}

export class BoardServicePWMSetRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getDutyCycle(): number;
  setDutyCycle(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServicePWMSetRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServicePWMSetRequest): BoardServicePWMSetRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServicePWMSetRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServicePWMSetRequest;
  static deserializeBinaryFromReader(message: BoardServicePWMSetRequest, reader: jspb.BinaryReader): BoardServicePWMSetRequest;
}

export namespace BoardServicePWMSetRequest {
  export type AsObject = {
    name: string,
    pin: string,
    dutyCycle: number,
  }
}

export class BoardServicePWMSetResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServicePWMSetResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServicePWMSetResponse): BoardServicePWMSetResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServicePWMSetResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServicePWMSetResponse;
  static deserializeBinaryFromReader(message: BoardServicePWMSetResponse, reader: jspb.BinaryReader): BoardServicePWMSetResponse;
}

export namespace BoardServicePWMSetResponse {
  export type AsObject = {
  }
}

export class BoardServicePWMSetFrequencyResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServicePWMSetFrequencyResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServicePWMSetFrequencyResponse): BoardServicePWMSetFrequencyResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServicePWMSetFrequencyResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServicePWMSetFrequencyResponse;
  static deserializeBinaryFromReader(message: BoardServicePWMSetFrequencyResponse, reader: jspb.BinaryReader): BoardServicePWMSetFrequencyResponse;
}

export namespace BoardServicePWMSetFrequencyResponse {
  export type AsObject = {
  }
}

export class BoardServicePWMSetFrequencyRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getFrequency(): number;
  setFrequency(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServicePWMSetFrequencyRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServicePWMSetFrequencyRequest): BoardServicePWMSetFrequencyRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServicePWMSetFrequencyRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServicePWMSetFrequencyRequest;
  static deserializeBinaryFromReader(message: BoardServicePWMSetFrequencyRequest, reader: jspb.BinaryReader): BoardServicePWMSetFrequencyRequest;
}

export namespace BoardServicePWMSetFrequencyRequest {
  export type AsObject = {
    name: string,
    pin: string,
    frequency: number,
  }
}

export class BoardServiceAnalogReaderReadRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getAnalogReaderName(): string;
  setAnalogReaderName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceAnalogReaderReadRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceAnalogReaderReadRequest): BoardServiceAnalogReaderReadRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceAnalogReaderReadRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceAnalogReaderReadRequest;
  static deserializeBinaryFromReader(message: BoardServiceAnalogReaderReadRequest, reader: jspb.BinaryReader): BoardServiceAnalogReaderReadRequest;
}

export namespace BoardServiceAnalogReaderReadRequest {
  export type AsObject = {
    boardName: string,
    analogReaderName: string,
  }
}

export class BoardServiceAnalogReaderReadResponse extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceAnalogReaderReadResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceAnalogReaderReadResponse): BoardServiceAnalogReaderReadResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceAnalogReaderReadResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceAnalogReaderReadResponse;
  static deserializeBinaryFromReader(message: BoardServiceAnalogReaderReadResponse, reader: jspb.BinaryReader): BoardServiceAnalogReaderReadResponse;
}

export namespace BoardServiceAnalogReaderReadResponse {
  export type AsObject = {
    value: number,
  }
}

export class DigitalInterruptConfig extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getType(): string;
  setType(value: string): void;

  getFormula(): string;
  setFormula(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): DigitalInterruptConfig.AsObject;
  static toObject(includeInstance: boolean, msg: DigitalInterruptConfig): DigitalInterruptConfig.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: DigitalInterruptConfig, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): DigitalInterruptConfig;
  static deserializeBinaryFromReader(message: DigitalInterruptConfig, reader: jspb.BinaryReader): DigitalInterruptConfig;
}

export namespace DigitalInterruptConfig {
  export type AsObject = {
    name: string,
    pin: string,
    type: string,
    formula: string,
  }
}

export class BoardServiceDigitalInterruptConfigRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getDigitalInterruptName(): string;
  setDigitalInterruptName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceDigitalInterruptConfigRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceDigitalInterruptConfigRequest): BoardServiceDigitalInterruptConfigRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceDigitalInterruptConfigRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceDigitalInterruptConfigRequest;
  static deserializeBinaryFromReader(message: BoardServiceDigitalInterruptConfigRequest, reader: jspb.BinaryReader): BoardServiceDigitalInterruptConfigRequest;
}

export namespace BoardServiceDigitalInterruptConfigRequest {
  export type AsObject = {
    boardName: string,
    digitalInterruptName: string,
  }
}

export class BoardServiceDigitalInterruptConfigResponse extends jspb.Message {
  hasConfig(): boolean;
  clearConfig(): void;
  getConfig(): DigitalInterruptConfig | undefined;
  setConfig(value?: DigitalInterruptConfig): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceDigitalInterruptConfigResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceDigitalInterruptConfigResponse): BoardServiceDigitalInterruptConfigResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceDigitalInterruptConfigResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceDigitalInterruptConfigResponse;
  static deserializeBinaryFromReader(message: BoardServiceDigitalInterruptConfigResponse, reader: jspb.BinaryReader): BoardServiceDigitalInterruptConfigResponse;
}

export namespace BoardServiceDigitalInterruptConfigResponse {
  export type AsObject = {
    config?: DigitalInterruptConfig.AsObject,
  }
}

export class BoardServiceDigitalInterruptValueRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getDigitalInterruptName(): string;
  setDigitalInterruptName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceDigitalInterruptValueRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceDigitalInterruptValueRequest): BoardServiceDigitalInterruptValueRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceDigitalInterruptValueRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceDigitalInterruptValueRequest;
  static deserializeBinaryFromReader(message: BoardServiceDigitalInterruptValueRequest, reader: jspb.BinaryReader): BoardServiceDigitalInterruptValueRequest;
}

export namespace BoardServiceDigitalInterruptValueRequest {
  export type AsObject = {
    boardName: string,
    digitalInterruptName: string,
  }
}

export class BoardServiceDigitalInterruptValueResponse extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceDigitalInterruptValueResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceDigitalInterruptValueResponse): BoardServiceDigitalInterruptValueResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceDigitalInterruptValueResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceDigitalInterruptValueResponse;
  static deserializeBinaryFromReader(message: BoardServiceDigitalInterruptValueResponse, reader: jspb.BinaryReader): BoardServiceDigitalInterruptValueResponse;
}

export namespace BoardServiceDigitalInterruptValueResponse {
  export type AsObject = {
    value: number,
  }
}

export class BoardServiceDigitalInterruptTickRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getDigitalInterruptName(): string;
  setDigitalInterruptName(value: string): void;

  getHigh(): boolean;
  setHigh(value: boolean): void;

  getNanos(): number;
  setNanos(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceDigitalInterruptTickRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceDigitalInterruptTickRequest): BoardServiceDigitalInterruptTickRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceDigitalInterruptTickRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceDigitalInterruptTickRequest;
  static deserializeBinaryFromReader(message: BoardServiceDigitalInterruptTickRequest, reader: jspb.BinaryReader): BoardServiceDigitalInterruptTickRequest;
}

export namespace BoardServiceDigitalInterruptTickRequest {
  export type AsObject = {
    boardName: string,
    digitalInterruptName: string,
    high: boolean,
    nanos: number,
  }
}

export class BoardServiceDigitalInterruptTickResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceDigitalInterruptTickResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceDigitalInterruptTickResponse): BoardServiceDigitalInterruptTickResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceDigitalInterruptTickResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceDigitalInterruptTickResponse;
  static deserializeBinaryFromReader(message: BoardServiceDigitalInterruptTickResponse, reader: jspb.BinaryReader): BoardServiceDigitalInterruptTickResponse;
}

export namespace BoardServiceDigitalInterruptTickResponse {
  export type AsObject = {
  }
}

