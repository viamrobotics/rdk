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


var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var proto_api_common_v1_common_pb = require('../../../../proto/api/common/v1/common_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.component = {};
proto.proto.api.component.v1 = require('./board_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.BoardServiceClient =
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
proto.proto.api.component.v1.BoardServicePromiseClient =
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
 *   !proto.proto.api.component.v1.BoardServiceStatusRequest,
 *   !proto.proto.api.component.v1.BoardServiceStatusResponse>}
 */
const methodDescriptor_BoardService_Status = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/Status',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceStatusRequest,
  proto.proto.api.component.v1.BoardServiceStatusResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceStatusRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceStatusResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceStatusRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceStatusResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceStatusResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.status =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/Status',
      request,
      metadata || {},
      methodDescriptor_BoardService_Status,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceStatusRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceStatusResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.status =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/Status',
      request,
      metadata || {},
      methodDescriptor_BoardService_Status);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceSetGPIORequest,
 *   !proto.proto.api.component.v1.BoardServiceSetGPIOResponse>}
 */
const methodDescriptor_BoardService_SetGPIO = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/SetGPIO',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceSetGPIORequest,
  proto.proto.api.component.v1.BoardServiceSetGPIOResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceSetGPIORequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceSetGPIOResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceSetGPIORequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceSetGPIOResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceSetGPIOResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.setGPIO =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/SetGPIO',
      request,
      metadata || {},
      methodDescriptor_BoardService_SetGPIO,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceSetGPIORequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceSetGPIOResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.setGPIO =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/SetGPIO',
      request,
      metadata || {},
      methodDescriptor_BoardService_SetGPIO);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceGetGPIORequest,
 *   !proto.proto.api.component.v1.BoardServiceGetGPIOResponse>}
 */
const methodDescriptor_BoardService_GetGPIO = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/GetGPIO',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceGetGPIORequest,
  proto.proto.api.component.v1.BoardServiceGetGPIOResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceGetGPIORequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceGetGPIOResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceGetGPIORequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceGetGPIOResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceGetGPIOResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.getGPIO =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/GetGPIO',
      request,
      metadata || {},
      methodDescriptor_BoardService_GetGPIO,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceGetGPIORequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceGetGPIOResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.getGPIO =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/GetGPIO',
      request,
      metadata || {},
      methodDescriptor_BoardService_GetGPIO);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceSetPWMRequest,
 *   !proto.proto.api.component.v1.BoardServiceSetPWMResponse>}
 */
const methodDescriptor_BoardService_SetPWM = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/SetPWM',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceSetPWMRequest,
  proto.proto.api.component.v1.BoardServiceSetPWMResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceSetPWMRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceSetPWMResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceSetPWMRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceSetPWMResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceSetPWMResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.setPWM =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/SetPWM',
      request,
      metadata || {},
      methodDescriptor_BoardService_SetPWM,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceSetPWMRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceSetPWMResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.setPWM =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/SetPWM',
      request,
      metadata || {},
      methodDescriptor_BoardService_SetPWM);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceSetPWMFrequencyRequest,
 *   !proto.proto.api.component.v1.BoardServiceSetPWMFrequencyResponse>}
 */
const methodDescriptor_BoardService_SetPWMFrequency = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/SetPWMFrequency',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceSetPWMFrequencyRequest,
  proto.proto.api.component.v1.BoardServiceSetPWMFrequencyResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceSetPWMFrequencyRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceSetPWMFrequencyResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceSetPWMFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceSetPWMFrequencyResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceSetPWMFrequencyResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.setPWMFrequency =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/SetPWMFrequency',
      request,
      metadata || {},
      methodDescriptor_BoardService_SetPWMFrequency,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceSetPWMFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceSetPWMFrequencyResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.setPWMFrequency =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/SetPWMFrequency',
      request,
      metadata || {},
      methodDescriptor_BoardService_SetPWMFrequency);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceReadAnalogReaderRequest,
 *   !proto.proto.api.component.v1.BoardServiceReadAnalogReaderResponse>}
 */
const methodDescriptor_BoardService_ReadAnalogReader = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/ReadAnalogReader',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceReadAnalogReaderRequest,
  proto.proto.api.component.v1.BoardServiceReadAnalogReaderResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceReadAnalogReaderRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceReadAnalogReaderResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceReadAnalogReaderRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceReadAnalogReaderResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceReadAnalogReaderResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.readAnalogReader =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/ReadAnalogReader',
      request,
      metadata || {},
      methodDescriptor_BoardService_ReadAnalogReader,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceReadAnalogReaderRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceReadAnalogReaderResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.readAnalogReader =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/ReadAnalogReader',
      request,
      metadata || {},
      methodDescriptor_BoardService_ReadAnalogReader);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueRequest,
 *   !proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueResponse>}
 */
const methodDescriptor_BoardService_GetDigitalInterruptValue = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/GetDigitalInterruptValue',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueRequest,
  proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.getDigitalInterruptValue =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/GetDigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_BoardService_GetDigitalInterruptValue,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceGetDigitalInterruptValueResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.getDigitalInterruptValue =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/GetDigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_BoardService_GetDigitalInterruptValue);
};


module.exports = proto.proto.api.component.v1;

