/**
 * @fileoverview gRPC-Web generated client stub for proto.api.component.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js')

var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.component = {};
proto.proto.api.component.v1 = require('./motor_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.MotorServiceClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options.format = 'text';

  /**
   * @private @const {!grpc.web.GrpcWebClientBase} The client
   */
  this.client_ = new grpc.web.GrpcWebClientBase(options);

  /**
   * @private @const {string} The hostname
   */
  this.hostname_ = hostname;

};


/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.MotorServicePromiseClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options.format = 'text';

  /**
   * @private @const {!grpc.web.GrpcWebClientBase} The client
   */
  this.client_ = new grpc.web.GrpcWebClientBase(options);

  /**
   * @private @const {string} The hostname
   */
  this.hostname_ = hostname;

};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceGetPIDConfigRequest,
 *   !proto.proto.api.component.v1.MotorServiceGetPIDConfigResponse>}
 */
const methodDescriptor_MotorService_GetPIDConfig = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/GetPIDConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceGetPIDConfigRequest,
  proto.proto.api.component.v1.MotorServiceGetPIDConfigResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceGetPIDConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceGetPIDConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceGetPIDConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceGetPIDConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.getPIDConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_MotorService_GetPIDConfig,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceGetPIDConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.getPIDConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_MotorService_GetPIDConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceSetPIDConfigRequest,
 *   !proto.proto.api.component.v1.MotorServiceSetPIDConfigResponse>}
 */
const methodDescriptor_MotorService_SetPIDConfig = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/SetPIDConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceSetPIDConfigRequest,
  proto.proto.api.component.v1.MotorServiceSetPIDConfigResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceSetPIDConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceSetPIDConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceSetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceSetPIDConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceSetPIDConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.setPIDConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/SetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_MotorService_SetPIDConfig,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceSetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceSetPIDConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.setPIDConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/SetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_MotorService_SetPIDConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServicePIDStepRequest,
 *   !proto.proto.api.component.v1.MotorServicePIDStepResponse>}
 */
const methodDescriptor_MotorService_PIDStep = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/PIDStep',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.proto.api.component.v1.MotorServicePIDStepRequest,
  proto.proto.api.component.v1.MotorServicePIDStepResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServicePIDStepRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServicePIDStepResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServicePIDStepRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServicePIDStepResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.pIDStep =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.component.v1.MotorService/PIDStep',
      request,
      metadata || {},
      methodDescriptor_MotorService_PIDStep);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServicePIDStepRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServicePIDStepResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.pIDStep =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.component.v1.MotorService/PIDStep',
      request,
      metadata || {},
      methodDescriptor_MotorService_PIDStep);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceSetPowerRequest,
 *   !proto.proto.api.component.v1.MotorServiceSetPowerResponse>}
 */
const methodDescriptor_MotorService_SetPower = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/SetPower',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceSetPowerRequest,
  proto.proto.api.component.v1.MotorServiceSetPowerResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceSetPowerRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceSetPowerResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceSetPowerRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceSetPowerResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceSetPowerResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.setPower =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/SetPower',
      request,
      metadata || {},
      methodDescriptor_MotorService_SetPower,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceSetPowerRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceSetPowerResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.setPower =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/SetPower',
      request,
      metadata || {},
      methodDescriptor_MotorService_SetPower);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceGoRequest,
 *   !proto.proto.api.component.v1.MotorServiceGoResponse>}
 */
const methodDescriptor_MotorService_Go = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/Go',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceGoRequest,
  proto.proto.api.component.v1.MotorServiceGoResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceGoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceGoResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceGoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceGoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.go =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/Go',
      request,
      metadata || {},
      methodDescriptor_MotorService_Go,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceGoResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.go =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/Go',
      request,
      metadata || {},
      methodDescriptor_MotorService_Go);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceGoForRequest,
 *   !proto.proto.api.component.v1.MotorServiceGoForResponse>}
 */
const methodDescriptor_MotorService_GoFor = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/GoFor',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceGoForRequest,
  proto.proto.api.component.v1.MotorServiceGoForResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceGoForRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceGoForResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoForRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceGoForResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceGoForResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.goFor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GoFor',
      request,
      metadata || {},
      methodDescriptor_MotorService_GoFor,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoForRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceGoForResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.goFor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GoFor',
      request,
      metadata || {},
      methodDescriptor_MotorService_GoFor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceGoToRequest,
 *   !proto.proto.api.component.v1.MotorServiceGoToResponse>}
 */
const methodDescriptor_MotorService_GoTo = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/GoTo',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceGoToRequest,
  proto.proto.api.component.v1.MotorServiceGoToResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceGoToRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceGoToResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoToRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceGoToResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceGoToResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.goTo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GoTo',
      request,
      metadata || {},
      methodDescriptor_MotorService_GoTo,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoToRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceGoToResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.goTo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GoTo',
      request,
      metadata || {},
      methodDescriptor_MotorService_GoTo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceGoTillStopRequest,
 *   !proto.proto.api.component.v1.MotorServiceGoTillStopResponse>}
 */
const methodDescriptor_MotorService_GoTillStop = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/GoTillStop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceGoTillStopRequest,
  proto.proto.api.component.v1.MotorServiceGoTillStopResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceGoTillStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceGoTillStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoTillStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceGoTillStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceGoTillStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.goTillStop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GoTillStop',
      request,
      metadata || {},
      methodDescriptor_MotorService_GoTillStop,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceGoTillStopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceGoTillStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.goTillStop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/GoTillStop',
      request,
      metadata || {},
      methodDescriptor_MotorService_GoTillStop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceResetZeroPositionRequest,
 *   !proto.proto.api.component.v1.MotorServiceResetZeroPositionResponse>}
 */
const methodDescriptor_MotorService_ResetZeroPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/ResetZeroPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceResetZeroPositionRequest,
  proto.proto.api.component.v1.MotorServiceResetZeroPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceResetZeroPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceResetZeroPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceResetZeroPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceResetZeroPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceResetZeroPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.resetZeroPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/ResetZeroPosition',
      request,
      metadata || {},
      methodDescriptor_MotorService_ResetZeroPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceResetZeroPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceResetZeroPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.resetZeroPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/ResetZeroPosition',
      request,
      metadata || {},
      methodDescriptor_MotorService_ResetZeroPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServicePositionRequest,
 *   !proto.proto.api.component.v1.MotorServicePositionResponse>}
 */
const methodDescriptor_MotorService_Position = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/Position',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServicePositionRequest,
  proto.proto.api.component.v1.MotorServicePositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServicePositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServicePositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServicePositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServicePositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServicePositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.position =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/Position',
      request,
      metadata || {},
      methodDescriptor_MotorService_Position,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServicePositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServicePositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.position =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/Position',
      request,
      metadata || {},
      methodDescriptor_MotorService_Position);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServicePositionSupportedRequest,
 *   !proto.proto.api.component.v1.MotorServicePositionSupportedResponse>}
 */
const methodDescriptor_MotorService_PositionSupported = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/PositionSupported',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServicePositionSupportedRequest,
  proto.proto.api.component.v1.MotorServicePositionSupportedResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServicePositionSupportedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServicePositionSupportedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServicePositionSupportedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServicePositionSupportedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServicePositionSupportedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.positionSupported =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/PositionSupported',
      request,
      metadata || {},
      methodDescriptor_MotorService_PositionSupported,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServicePositionSupportedRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServicePositionSupportedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.positionSupported =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/PositionSupported',
      request,
      metadata || {},
      methodDescriptor_MotorService_PositionSupported);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceStopRequest,
 *   !proto.proto.api.component.v1.MotorServiceStopResponse>}
 */
const methodDescriptor_MotorService_Stop = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/Stop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceStopRequest,
  proto.proto.api.component.v1.MotorServiceStopResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/Stop',
      request,
      metadata || {},
      methodDescriptor_MotorService_Stop,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceStopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/Stop',
      request,
      metadata || {},
      methodDescriptor_MotorService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.MotorServiceIsOnRequest,
 *   !proto.proto.api.component.v1.MotorServiceIsOnResponse>}
 */
const methodDescriptor_MotorService_IsOn = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.MotorService/IsOn',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.MotorServiceIsOnRequest,
  proto.proto.api.component.v1.MotorServiceIsOnResponse,
  /**
   * @param {!proto.proto.api.component.v1.MotorServiceIsOnRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.MotorServiceIsOnResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.MotorServiceIsOnRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.MotorServiceIsOnResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.MotorServiceIsOnResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.MotorServiceClient.prototype.isOn =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/IsOn',
      request,
      metadata || {},
      methodDescriptor_MotorService_IsOn,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.MotorServiceIsOnRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.MotorServiceIsOnResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.MotorServicePromiseClient.prototype.isOn =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.MotorService/IsOn',
      request,
      metadata || {},
      methodDescriptor_MotorService_IsOn);
};


module.exports = proto.proto.api.component.v1;

