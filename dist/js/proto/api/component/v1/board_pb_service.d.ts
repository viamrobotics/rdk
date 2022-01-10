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

type BoardServiceGPIOSet = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceGPIOSetRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceGPIOSetResponse;
};

type BoardServiceGPIOGet = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceGPIOGetRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceGPIOGetResponse;
};

type BoardServicePWMSet = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServicePWMSetRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServicePWMSetResponse;
};

type BoardServicePWMSetFrequency = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyResponse;
};

type BoardServiceAnalogReaderRead = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadResponse;
};

type BoardServiceDigitalInterruptConfig = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigResponse;
};

type BoardServiceDigitalInterruptValue = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueResponse;
};

type BoardServiceDigitalInterruptTick = {
  readonly methodName: string;
  readonly service: typeof BoardService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickRequest;
  readonly responseType: typeof proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickResponse;
};

export class BoardService {
  static readonly serviceName: string;
  static readonly Status: BoardServiceStatus;
  static readonly GPIOSet: BoardServiceGPIOSet;
  static readonly GPIOGet: BoardServiceGPIOGet;
  static readonly PWMSet: BoardServicePWMSet;
  static readonly PWMSetFrequency: BoardServicePWMSetFrequency;
  static readonly AnalogReaderRead: BoardServiceAnalogReaderRead;
  static readonly DigitalInterruptConfig: BoardServiceDigitalInterruptConfig;
  static readonly DigitalInterruptValue: BoardServiceDigitalInterruptValue;
  static readonly DigitalInterruptTick: BoardServiceDigitalInterruptTick;
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
  gPIOSet(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGPIOSetRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGPIOSetResponse|null) => void
  ): UnaryResponse;
  gPIOSet(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGPIOSetRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGPIOSetResponse|null) => void
  ): UnaryResponse;
  gPIOGet(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGPIOGetRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGPIOGetResponse|null) => void
  ): UnaryResponse;
  gPIOGet(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceGPIOGetRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceGPIOGetResponse|null) => void
  ): UnaryResponse;
  pWMSet(
    requestMessage: proto_api_component_v1_board_pb.BoardServicePWMSetRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServicePWMSetResponse|null) => void
  ): UnaryResponse;
  pWMSet(
    requestMessage: proto_api_component_v1_board_pb.BoardServicePWMSetRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServicePWMSetResponse|null) => void
  ): UnaryResponse;
  pWMSetFrequency(
    requestMessage: proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyResponse|null) => void
  ): UnaryResponse;
  pWMSetFrequency(
    requestMessage: proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyResponse|null) => void
  ): UnaryResponse;
  analogReaderRead(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadResponse|null) => void
  ): UnaryResponse;
  analogReaderRead(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadResponse|null) => void
  ): UnaryResponse;
  digitalInterruptConfig(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigResponse|null) => void
  ): UnaryResponse;
  digitalInterruptConfig(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigResponse|null) => void
  ): UnaryResponse;
  digitalInterruptValue(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueResponse|null) => void
  ): UnaryResponse;
  digitalInterruptValue(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueResponse|null) => void
  ): UnaryResponse;
  digitalInterruptTick(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickResponse|null) => void
  ): UnaryResponse;
  digitalInterruptTick(
    requestMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickResponse|null) => void
  ): UnaryResponse;
}

