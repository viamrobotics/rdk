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

  getDutyCyclePct(): number;
  setDutyCyclePct(value: number): void;

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
    dutyCyclePct: number,
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

  getFrequencyHz(): number;
  setFrequencyHz(value: number): void;

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
    frequencyHz: number,
  }
}

export class BoardServiceReadAnalogReaderRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getAnalogReaderName(): string;
  setAnalogReaderName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceReadAnalogReaderRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceReadAnalogReaderRequest): BoardServiceReadAnalogReaderRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceReadAnalogReaderRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceReadAnalogReaderRequest;
  static deserializeBinaryFromReader(message: BoardServiceReadAnalogReaderRequest, reader: jspb.BinaryReader): BoardServiceReadAnalogReaderRequest;
}

export namespace BoardServiceReadAnalogReaderRequest {
  export type AsObject = {
    boardName: string,
    analogReaderName: string,
  }
}

export class BoardServiceReadAnalogReaderResponse extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceReadAnalogReaderResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceReadAnalogReaderResponse): BoardServiceReadAnalogReaderResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceReadAnalogReaderResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceReadAnalogReaderResponse;
  static deserializeBinaryFromReader(message: BoardServiceReadAnalogReaderResponse, reader: jspb.BinaryReader): BoardServiceReadAnalogReaderResponse;
}

export namespace BoardServiceReadAnalogReaderResponse {
  export type AsObject = {
    value: number,
  }
}

export class BoardServiceGetDigitalInterruptValueRequest extends jspb.Message {
  getBoardName(): string;
  setBoardName(value: string): void;

  getDigitalInterruptName(): string;
  setDigitalInterruptName(value: string): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGetDigitalInterruptValueRequest.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGetDigitalInterruptValueRequest): BoardServiceGetDigitalInterruptValueRequest.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGetDigitalInterruptValueRequest, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGetDigitalInterruptValueRequest;
  static deserializeBinaryFromReader(message: BoardServiceGetDigitalInterruptValueRequest, reader: jspb.BinaryReader): BoardServiceGetDigitalInterruptValueRequest;
}

export namespace BoardServiceGetDigitalInterruptValueRequest {
  export type AsObject = {
    boardName: string,
    digitalInterruptName: string,
  }
}

export class BoardServiceGetDigitalInterruptValueResponse extends jspb.Message {
  getValue(): number;
  setValue(value: number): void;

  serializeBinary(): Uint8Array;
  toObject(includeInstance?: boolean): BoardServiceGetDigitalInterruptValueResponse.AsObject;
  static toObject(includeInstance: boolean, msg: BoardServiceGetDigitalInterruptValueResponse): BoardServiceGetDigitalInterruptValueResponse.AsObject;
  static extensions: {[key: number]: jspb.ExtensionFieldInfo<jspb.Message>};
  static extensionsBinary: {[key: number]: jspb.ExtensionFieldBinaryInfo<jspb.Message>};
  static serializeBinaryToWriter(message: BoardServiceGetDigitalInterruptValueResponse, writer: jspb.BinaryWriter): void;
  static deserializeBinary(bytes: Uint8Array): BoardServiceGetDigitalInterruptValueResponse;
  static deserializeBinaryFromReader(message: BoardServiceGetDigitalInterruptValueResponse, reader: jspb.BinaryReader): BoardServiceGetDigitalInterruptValueResponse;
}

export namespace BoardServiceGetDigitalInterruptValueResponse {
  export type AsObject = {
    value: number,
  }
}

