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

export class BoardServiceSetGPIORequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getHigh(): boolean;
  setHigh(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceSetGPIORequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceSetGPIORequest): BoardServiceSetGPIORequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceSetGPIORequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceSetGPIORequest;
  static deserializeBinaryFromReader(message: BoardServiceSetGPIORequest, reader: jspb.BinaryReader): BoardServiceSetGPIORequest;
}

export namespace BoardServiceSetGPIORequest {
  export type AsObject = {
    name: string,
    pin: string,
    high: boolean,
  }
}

export class BoardServiceSetGPIOResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceSetGPIOResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceSetGPIOResponse): BoardServiceSetGPIOResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceSetGPIOResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceSetGPIOResponse;
  static deserializeBinaryFromReader(message: BoardServiceSetGPIOResponse, reader: jspb.BinaryReader): BoardServiceSetGPIOResponse;
}

export namespace BoardServiceSetGPIOResponse {
  export type AsObject = {
  }
}

export class BoardServiceGetGPIORequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGetGPIORequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGetGPIORequest): BoardServiceGetGPIORequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGetGPIORequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGetGPIORequest;
  static deserializeBinaryFromReader(message: BoardServiceGetGPIORequest, reader: jspb.BinaryReader): BoardServiceGetGPIORequest;
}

export namespace BoardServiceGetGPIORequest {
  export type AsObject = {
    name: string,
    pin: string,
  }
}

export class BoardServiceGetGPIOResponse extends jspb.Message {
  getHigh(): boolean;
  setHigh(value: boolean): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGetGPIOResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGetGPIOResponse): BoardServiceGetGPIOResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGetGPIOResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGetGPIOResponse;
  static deserializeBinaryFromReader(message: BoardServiceGetGPIOResponse, reader: jspb.BinaryReader): BoardServiceGetGPIOResponse;
}

export namespace BoardServiceGetGPIOResponse {
  export type AsObject = {
    high: boolean,
  }
}

export class BoardServiceSetPWMRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getDutyCycle(): number;
  setDutyCycle(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceSetPWMRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceSetPWMRequest): BoardServiceSetPWMRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceSetPWMRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceSetPWMRequest;
  static deserializeBinaryFromReader(message: BoardServiceSetPWMRequest, reader: jspb.BinaryReader): BoardServiceSetPWMRequest;
}

export namespace BoardServiceSetPWMRequest {
  export type AsObject = {
    name: string,
    pin: string,
    dutyCycle: number,
  }
}

export class BoardServiceSetPWMResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceSetPWMResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceSetPWMResponse): BoardServiceSetPWMResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceSetPWMResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceSetPWMResponse;
  static deserializeBinaryFromReader(message: BoardServiceSetPWMResponse, reader: jspb.BinaryReader): BoardServiceSetPWMResponse;
}

export namespace BoardServiceSetPWMResponse {
  export type AsObject = {
  }
}

export class BoardServiceSetPWMFrequencyResponse extends jspb.Message {
  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceSetPWMFrequencyResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceSetPWMFrequencyResponse): BoardServiceSetPWMFrequencyResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceSetPWMFrequencyResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceSetPWMFrequencyResponse;
  static deserializeBinaryFromReader(message: BoardServiceSetPWMFrequencyResponse, reader: jspb.BinaryReader): BoardServiceSetPWMFrequencyResponse;
}

export namespace BoardServiceSetPWMFrequencyResponse {
  export type AsObject = {
  }
}

export class BoardServiceSetPWMFrequencyRequest extends jspb.Message {
  getName(): string;
  setName(value: string): void;

  getPin(): string;
  setPin(value: string): void;

  getFrequency(): number;
  setFrequency(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceSetPWMFrequencyRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceSetPWMFrequencyRequest): BoardServiceSetPWMFrequencyRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceSetPWMFrequencyRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceSetPWMFrequencyRequest;
  static deserializeBinaryFromReader(message: BoardServiceSetPWMFrequencyRequest, reader: jspb.BinaryReader): BoardServiceSetPWMFrequencyRequest;
}

export namespace BoardServiceSetPWMFrequencyRequest {
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

