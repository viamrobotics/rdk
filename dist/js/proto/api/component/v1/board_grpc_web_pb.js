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
 *   !proto.proto.api.component.v1.BoardServicePWMSetRequest,
 *   !proto.proto.api.component.v1.BoardServicePWMSetResponse>}
 */
const methodDescriptor_BoardService_PWMSet = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/PWMSet',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServicePWMSetRequest,
  proto.proto.api.component.v1.BoardServicePWMSetResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServicePWMSetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServicePWMSetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServicePWMSetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServicePWMSetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServicePWMSetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.pWMSet =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/PWMSet',
      request,
      metadata || {},
      methodDescriptor_BoardService_PWMSet,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServicePWMSetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServicePWMSetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.pWMSet =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/PWMSet',
      request,
      metadata || {},
      methodDescriptor_BoardService_PWMSet);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServicePWMSetFrequencyRequest,
 *   !proto.proto.api.component.v1.BoardServicePWMSetFrequencyResponse>}
 */
const methodDescriptor_BoardService_PWMSetFrequency = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/PWMSetFrequency',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServicePWMSetFrequencyRequest,
  proto.proto.api.component.v1.BoardServicePWMSetFrequencyResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServicePWMSetFrequencyRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServicePWMSetFrequencyResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServicePWMSetFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServicePWMSetFrequencyResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServicePWMSetFrequencyResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.pWMSetFrequency =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/PWMSetFrequency',
      request,
      metadata || {},
      methodDescriptor_BoardService_PWMSetFrequency,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServicePWMSetFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServicePWMSetFrequencyResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.pWMSetFrequency =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/PWMSetFrequency',
      request,
      metadata || {},
      methodDescriptor_BoardService_PWMSetFrequency);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceAnalogReaderReadRequest,
 *   !proto.proto.api.component.v1.BoardServiceAnalogReaderReadResponse>}
 */
const methodDescriptor_BoardService_AnalogReaderRead = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/AnalogReaderRead',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceAnalogReaderReadRequest,
  proto.proto.api.component.v1.BoardServiceAnalogReaderReadResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceAnalogReaderReadRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceAnalogReaderReadResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceAnalogReaderReadRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceAnalogReaderReadResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceAnalogReaderReadResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.analogReaderRead =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/AnalogReaderRead',
      request,
      metadata || {},
      methodDescriptor_BoardService_AnalogReaderRead,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceAnalogReaderReadRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceAnalogReaderReadResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.analogReaderRead =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/AnalogReaderRead',
      request,
      metadata || {},
      methodDescriptor_BoardService_AnalogReaderRead);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigRequest,
 *   !proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigResponse>}
 */
const methodDescriptor_BoardService_DigitalInterruptConfig = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/DigitalInterruptConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigRequest,
  proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.digitalInterruptConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/DigitalInterruptConfig',
      request,
      metadata || {},
      methodDescriptor_BoardService_DigitalInterruptConfig,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceDigitalInterruptConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.digitalInterruptConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/DigitalInterruptConfig',
      request,
      metadata || {},
      methodDescriptor_BoardService_DigitalInterruptConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceDigitalInterruptValueRequest,
 *   !proto.proto.api.component.v1.BoardServiceDigitalInterruptValueResponse>}
 */
const methodDescriptor_BoardService_DigitalInterruptValue = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/DigitalInterruptValue',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceDigitalInterruptValueRequest,
  proto.proto.api.component.v1.BoardServiceDigitalInterruptValueResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptValueRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceDigitalInterruptValueResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceDigitalInterruptValueResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceDigitalInterruptValueResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.digitalInterruptValue =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/DigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_BoardService_DigitalInterruptValue,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceDigitalInterruptValueResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.digitalInterruptValue =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/DigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_BoardService_DigitalInterruptValue);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BoardServiceDigitalInterruptTickRequest,
 *   !proto.proto.api.component.v1.BoardServiceDigitalInterruptTickResponse>}
 */
const methodDescriptor_BoardService_DigitalInterruptTick = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BoardService/DigitalInterruptTick',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BoardServiceDigitalInterruptTickRequest,
  proto.proto.api.component.v1.BoardServiceDigitalInterruptTickResponse,
  /**
   * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptTickRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BoardServiceDigitalInterruptTickResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptTickRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BoardServiceDigitalInterruptTickResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BoardServiceDigitalInterruptTickResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BoardServiceClient.prototype.digitalInterruptTick =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/DigitalInterruptTick',
      request,
      metadata || {},
      methodDescriptor_BoardService_DigitalInterruptTick,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BoardServiceDigitalInterruptTickRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BoardServiceDigitalInterruptTickResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BoardServicePromiseClient.prototype.digitalInterruptTick =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BoardService/DigitalInterruptTick',
      request,
      metadata || {},
      methodDescriptor_BoardService_DigitalInterruptTick);
};


module.exports = proto.proto.api.component.v1;

