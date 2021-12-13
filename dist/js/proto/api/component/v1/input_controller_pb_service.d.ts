// package: proto.api.component.v1
// file: proto/api/component/v1/input_controller.proto

import * as proto_api_component_v1_input_controller_pb from "../../../../proto/api/component/v1/input_controller_pb";
import {grpc} from "@improbable-eng/grpc-web";

type InputControllerServiceControls = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceControlsRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceControlsResponse;
};

type InputControllerServiceLastEvents = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsResponse;
};

type InputControllerServiceEventStream = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceEventStreamRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceEventStreamResponse;
};

type InputControllerServiceInjectEvent = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventResponse;
};

export class InputControllerService {
  static readonly serviceName: string;
  static readonly Controls: InputControllerServiceControls;
  static readonly LastEvents: InputControllerServiceLastEvents;
  static readonly EventStream: InputControllerServiceEventStream;
  static readonly InjectEvent: InputControllerServiceInjectEvent;
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

export class InputControllerServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  controls(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceControlsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceControlsResponse|null) => void
  ): UnaryResponse;
  controls(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceControlsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceControlsResponse|null) => void
  ): UnaryResponse;
  lastEvents(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsResponse|null) => void
  ): UnaryResponse;
  lastEvents(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsResponse|null) => void
  ): UnaryResponse;
  eventStream(requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceEventStreamRequest, metadata?: grpc.Metadata): ResponseStream<proto_api_component_v1_input_controller_pb.InputControllerServiceEventStreamResponse>;
  injectEvent(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventResponse|null) => void
  ): UnaryResponse;
  injectEvent(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventResponse|null) => void
  ): UnaryResponse;
}

