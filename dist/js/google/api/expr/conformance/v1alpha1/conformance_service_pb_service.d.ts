// package: google.api.expr.conformance.v1alpha1
// file: google/api/expr/conformance/v1alpha1/conformance_service.proto

import * as google_api_expr_conformance_v1alpha1_conformance_service_pb from "../../../../../google/api/expr/conformance/v1alpha1/conformance_service_pb";
import {grpc} from "@improbable-eng/grpc-web";

type ConformanceServiceParse = {
  readonly methodName: string;
  readonly service: typeof ConformanceService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseRequest;
  readonly responseType: typeof google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseResponse;
};

type ConformanceServiceCheck = {
  readonly methodName: string;
  readonly service: typeof ConformanceService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckRequest;
  readonly responseType: typeof google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckResponse;
};

type ConformanceServiceEval = {
  readonly methodName: string;
  readonly service: typeof ConformanceService;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalRequest;
  readonly responseType: typeof google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalResponse;
};

export class ConformanceService {
  static readonly serviceName: string;
  static readonly Parse: ConformanceServiceParse;
  static readonly Check: ConformanceServiceCheck;
  static readonly Eval: ConformanceServiceEval;
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

export class ConformanceServiceClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  parse(
    requestMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseResponse|null) => void
  ): UnaryResponse;
  parse(
    requestMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseResponse|null) => void
  ): UnaryResponse;
  check(
    requestMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckResponse|null) => void
  ): UnaryResponse;
  check(
    requestMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckResponse|null) => void
  ): UnaryResponse;
  eval(
    requestMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalResponse|null) => void
  ): UnaryResponse;
  eval(
    requestMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalResponse|null) => void
  ): UnaryResponse;
}

