// package: proto.api.component.v1
// file: proto/api/component/v1/input_controller.proto

import * as proto_api_component_v1_input_controller_pb from "../../../../proto/api/component/v1/input_controller_pb";
import {grpc} from "@improbable-eng/grpc-web";

type InputControllerServiceGetControls = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsResponse;
};

type InputControllerServiceGetEvents = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsResponse;
};

type InputControllerServiceStreamEvents = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: true;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceStreamEventsRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceStreamEventsResponse;
};

type InputControllerServiceTriggerEvent = {
  readonly methodName: string;
  readonly service: typeof InputControllerService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventRequest;
  readonly responseType: typeof proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventResponse;
};

export class InputControllerService {
  static readonly serviceName: string;
  static readonly GetControls: InputControllerServiceGetControls;
  static readonly GetEvents: InputControllerServiceGetEvents;
  static readonly StreamEvents: InputControllerServiceStreamEvents;
  static readonly TriggerEvent: InputControllerServiceTriggerEvent;
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
  getControls(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsResponse|null) => void
  ): UnaryResponse;
  getControls(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsResponse|null) => void
  ): UnaryResponse;
  getEvents(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsResponse|null) => void
  ): UnaryResponse;
  getEvents(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsResponse|null) => void
  ): UnaryResponse;
  streamEvents(requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceStreamEventsRequest, metadata?: grpc.Metadata): ResponseStream<proto_api_component_v1_input_controller_pb.InputControllerServiceStreamEventsResponse>;
  triggerEvent(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventResponse|null) => void
  ): UnaryResponse;
  triggerEvent(
    requestMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventRequest,
    callback: (error: ServiceError|null, responseMessage: proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventResponse|null) => void
  ): UnaryResponse;
}

