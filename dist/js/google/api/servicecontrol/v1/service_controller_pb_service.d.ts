// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/service_controller.proto

import * as google_api_servicecontrol_v1_service_controller_pb from "../../../../google/api/servicecontrol/v1/service_controller_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ServiceControllerCheck = {
  readonly methodName: string;
  readonly service: typeof ServiceController;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicecontrol_v1_service_controller_pb.CheckRequest;
  readonly responseType: typeof google_api_servicecontrol_v1_service_controller_pb.CheckResponse;
};

type ServiceControllerReport = {
  readonly methodName: string;
  readonly service: typeof ServiceController;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicecontrol_v1_service_controller_pb.ReportRequest;
  readonly responseType: typeof google_api_servicecontrol_v1_service_controller_pb.ReportResponse;
};

export class ServiceController {
  static readonly serviceName: string;
  static readonly Check: ServiceControllerCheck;
  static readonly Report: ServiceControllerReport;
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

export class ServiceControllerClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  check(
    requestMessage: google_api_servicecontrol_v1_service_controller_pb.CheckRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicecontrol_v1_service_controller_pb.CheckResponse|null) => void
  ): UnaryResponse;
  check(
    requestMessage: google_api_servicecontrol_v1_service_controller_pb.CheckRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicecontrol_v1_service_controller_pb.CheckResponse|null) => void
  ): UnaryResponse;
  report(
    requestMessage: google_api_servicecontrol_v1_service_controller_pb.ReportRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicecontrol_v1_service_controller_pb.ReportResponse|null) => void
  ): UnaryResponse;
  report(
    requestMessage: google_api_servicecontrol_v1_service_controller_pb.ReportRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicecontrol_v1_service_controller_pb.ReportResponse|null) => void
  ): UnaryResponse;
}

