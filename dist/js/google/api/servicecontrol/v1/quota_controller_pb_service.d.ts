// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/quota_controller.proto

import * as google_api_servicecontrol_v1_quota_controller_pb from "../../../../google/api/servicecontrol/v1/quota_controller_pb";
import {grpc} from "@improbable-eng/grpc-web";

type QuotaControllerAllocateQuota = {
  readonly methodName: string;
  readonly service: typeof QuotaController;
  readonly requestStream: false;
  readonly responseStream: false;
  readonly requestType: typeof google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaRequest;
  readonly responseType: typeof google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaResponse;
};

export class QuotaController {
  static readonly serviceName: string;
  static readonly AllocateQuota: QuotaControllerAllocateQuota;
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

export class QuotaControllerClient {
  readonly serviceHost: string;

  constructor(serviceHost: string, options?: grpc.RpcOptions);
  allocateQuota(
    requestMessage: google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaRequest,
    metadata: grpc.Metadata,
    callback: (error: ServiceError|null, responseMessage: google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaResponse|null) => void
  ): UnaryResponse;
  allocateQuota(
    requestMessage: google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaRequest,
    callback: (error: ServiceError|null, responseMessage: google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaResponse|null) => void
  ): UnaryResponse;
}

