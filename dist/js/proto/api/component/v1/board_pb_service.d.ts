// package: proto.api.component.v1
// file: proto/api/component/v1/board.proto

import * as proto_api_component_v1_board_pb from "../../../../proto/api/component/v1/board_pb";
import {grpc} from "@improbable-eng/grpc-web";

type BoardServiceStatus = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceStatusRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceStatusResponse;
};

type BoardServiceSetGPIO = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceSetGPIORequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceSetGPIOResponse;
};

type BoardServiceGetGPIO = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceGetGPIORequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceGetGPIOResponse;
};

type BoardServiceSetPWM = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceSetPWMRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceSetPWMResponse;
};

type BoardServiceSetPWMFrequency = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyResponse;
};

type BoardServiceReadAnalogReader = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderResponse;
};

type BoardServiceGetDigitalInterruptValue = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceGetDigitalInterruptValueRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceGetDigitalInterruptValueResponse;
};

export class BoardService {
  static readonly serviceName: string;
  static readonly Status: BoardServiceStatus;
  static readonly SetGPIO: BoardServiceSetGPIO;
  static readonly GetGPIO: BoardServiceGetGPIO;
  static readonly SetPWM: BoardServiceSetPWM;
  static readonly SetPWMFrequency: BoardServiceSetPWMFrequency;
  static readonly ReadAnalogReader: BoardServiceReadAnalogReader;
  static readonly GetDigitalInterruptValue: BoardServiceGetDigitalInterruptValue;
}

export type ServiceError = { message: string, code: number; metadata: grpc.Metadata }
export type Status = { details: string, code: number; metadata: grpc.Metadata }

interface UnaryResponse {
  cancel(): void;
}
interface ResponseStream<T> {
  cancel(): void;
  on(type: 'data', handler: (message: T) => void): ResponseStream<T>;
  on(type: 'end', handler: (status?: Status) => void): ResponseStream<T>;
  on(type: 'status', handler: (status: Status) => void): ResponseStream<T>;
}
interface RequestStream<T> {
  write(message: T): RequestStream<T>;
  end(): void;
  cancel(): void;
  on(type: 'end', handler: (status?: Status) => void): RequestStream<T>;
  on(type: 'status', handler: (status: Status) => void): RequestStream<T>;
}
interface BidirectionalStream<ReqT, ResT> {
  write(message: ReqT): BidirectionalStream<ReqT, ResT>;
  end(): void;
  cancel(): void;
  on(type: 'data', handler: (message: ResT) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'end', handler: (status?: Status) => void): BidirectionalStream<ReqT, ResT>;
  on(type: 'status', handler: (status: Status) => void): BidirectionalStream<ReqT, ResT>;
}

export class BoardServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  status(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceStatusRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceStatusResponse|null) => void
  ): UnaryResponse;
  status(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceStatusRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceStatusResponse|null) => void
  ): UnaryResponse;
  setGPIO(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceSetGPIORequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceSetGPIOResponse|null) => void
  ): UnaryResponse;
  setGPIO(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceSetGPIORequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceSetGPIOResponse|null) => void
  ): UnaryResponse;
  getGPIO(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGetGPIORequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGetGPIOResponse|null) => void
  ): UnaryResponse;
  getGPIO(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGetGPIORequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGetGPIOResponse|null) => void
  ): UnaryResponse;
  setPWM(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMResponse|null) => void
  ): UnaryResponse;
  setPWM(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMResponse|null) => void
  ): UnaryResponse;
  setPWMFrequency(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyResponse|null) => void
  ): UnaryResponse;
  setPWMFrequency(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyResponse|null) => void
  ): UnaryResponse;
  readAnalogReader(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderResponse|null) => void
  ): UnaryResponse;
  readAnalogReader(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderResponse|null) => void
  ): UnaryResponse;
  getDigitalInterruptValue(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGetDigitalInterruptValueRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGetDigitalInterruptValueResponse|null) => void
  ): UnaryResponse;
  getDigitalInterruptValue(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGetDigitalInterruptValueRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGetDigitalInterruptValueResponse|null) => void
  ): UnaryResponse;
}

