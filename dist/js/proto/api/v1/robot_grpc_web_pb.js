/**
 * @fileoverview gRPC-Web generated client stub for proto.api.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js')

var google_protobuf_duration_pb = require('google-protobuf/google/protobuf/duration_pb.js')

var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js')

var google_api_annotations_pb = require('../../../google/api/annotations_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.v1 = require('./robot_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.v1.RobotServiceClient =
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
proto.proto.api.v1.RobotServicePromiseClient =
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
 *   !proto.proto.api.v1.StatusRequest,
 *   !proto.proto.api.v1.StatusResponse>}
 */
const methodDescriptor_RobotService_Status = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/Status',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.StatusRequest,
  proto.proto.api.v1.StatusResponse,
  /**
   * @param {!proto.proto.api.v1.StatusRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.StatusResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.StatusRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.StatusResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.StatusResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.status =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/Status',
      request,
      metadata || {},
      methodDescriptor_RobotService_Status,
      callback);
};


/**
 * @param {!proto.proto.api.v1.StatusRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.StatusResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.status =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/Status',
      request,
      metadata || {},
      methodDescriptor_RobotService_Status);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.StatusStreamRequest,
 *   !proto.proto.api.v1.StatusStreamResponse>}
 */
const methodDescriptor_RobotService_StatusStream = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/StatusStream',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.proto.api.v1.StatusStreamRequest,
  proto.proto.api.v1.StatusStreamResponse,
  /**
   * @param {!proto.proto.api.v1.StatusStreamRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.StatusStreamResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.StatusStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.StatusStreamResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.statusStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/StatusStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_StatusStream);
};


/**
 * @param {!proto.proto.api.v1.StatusStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.StatusStreamResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.statusStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/StatusStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_StatusStream);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ConfigRequest,
 *   !proto.proto.api.v1.ConfigResponse>}
 */
const methodDescriptor_RobotService_Config = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/Config',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ConfigRequest,
  proto.proto.api.v1.ConfigResponse,
  /**
   * @param {!proto.proto.api.v1.ConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.config =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/Config',
      request,
      metadata || {},
      methodDescriptor_RobotService_Config,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.config =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/Config',
      request,
      metadata || {},
      methodDescriptor_RobotService_Config);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.DoActionRequest,
 *   !proto.proto.api.v1.DoActionResponse>}
 */
const methodDescriptor_RobotService_DoAction = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/DoAction',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.DoActionRequest,
  proto.proto.api.v1.DoActionResponse,
  /**
   * @param {!proto.proto.api.v1.DoActionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.DoActionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.DoActionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.DoActionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.DoActionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.doAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/DoAction',
      request,
      metadata || {},
      methodDescriptor_RobotService_DoAction,
      callback);
};


/**
 * @param {!proto.proto.api.v1.DoActionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.DoActionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.doAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/DoAction',
      request,
      metadata || {},
      methodDescriptor_RobotService_DoAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseMoveStraightRequest,
 *   !proto.proto.api.v1.BaseMoveStraightResponse>}
 */
const methodDescriptor_RobotService_BaseMoveStraight = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseMoveStraight',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseMoveStraightRequest,
  proto.proto.api.v1.BaseMoveStraightResponse,
  /**
   * @param {!proto.proto.api.v1.BaseMoveStraightRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseMoveStraightResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseMoveStraightRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseMoveStraightResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseMoveStraightResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseMoveStraight =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseMoveStraight',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseMoveStraight,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseMoveStraightRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseMoveStraightResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseMoveStraight =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseMoveStraight',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseMoveStraight);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseMoveArcRequest,
 *   !proto.proto.api.v1.BaseMoveArcResponse>}
 */
const methodDescriptor_RobotService_BaseMoveArc = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseMoveArc',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseMoveArcRequest,
  proto.proto.api.v1.BaseMoveArcResponse,
  /**
   * @param {!proto.proto.api.v1.BaseMoveArcRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseMoveArcResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseMoveArcRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseMoveArcResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseMoveArcResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseMoveArc =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseMoveArc',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseMoveArc,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseMoveArcRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseMoveArcResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseMoveArc =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseMoveArc',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseMoveArc);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseSpinRequest,
 *   !proto.proto.api.v1.BaseSpinResponse>}
 */
const methodDescriptor_RobotService_BaseSpin = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseSpin',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseSpinRequest,
  proto.proto.api.v1.BaseSpinResponse,
  /**
   * @param {!proto.proto.api.v1.BaseSpinRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseSpinResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseSpinRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseSpinResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseSpinResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseSpin =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseSpin',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseSpin,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseSpinRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseSpinResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseSpin =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseSpin',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseSpin);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseStopRequest,
 *   !proto.proto.api.v1.BaseStopResponse>}
 */
const methodDescriptor_RobotService_BaseStop = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseStop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseStopRequest,
  proto.proto.api.v1.BaseStopResponse,
  /**
   * @param {!proto.proto.api.v1.BaseStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseStop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseStop,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseStopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseStop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseStop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseWidthMillisRequest,
 *   !proto.proto.api.v1.BaseWidthMillisResponse>}
 */
const methodDescriptor_RobotService_BaseWidthMillis = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseWidthMillis',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseWidthMillisRequest,
  proto.proto.api.v1.BaseWidthMillisResponse,
  /**
   * @param {!proto.proto.api.v1.BaseWidthMillisRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseWidthMillisResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseWidthMillisRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseWidthMillisResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseWidthMillisResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseWidthMillis =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseWidthMillis',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseWidthMillis,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseWidthMillisRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseWidthMillisResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseWidthMillis =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseWidthMillis',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseWidthMillis);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardStatusRequest,
 *   !proto.proto.api.v1.BoardStatusResponse>}
 */
const methodDescriptor_RobotService_BoardStatus = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardStatus',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardStatusRequest,
  proto.proto.api.v1.BoardStatusResponse,
  /**
   * @param {!proto.proto.api.v1.BoardStatusRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardStatusResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardStatusRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardStatusResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardStatusResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardStatus =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardStatus',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardStatus,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardStatusRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardStatusResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardStatus =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardStatus',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardStatus);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardGPIOSetRequest,
 *   !proto.proto.api.v1.BoardGPIOSetResponse>}
 */
const methodDescriptor_RobotService_BoardGPIOSet = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardGPIOSet',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardGPIOSetRequest,
  proto.proto.api.v1.BoardGPIOSetResponse,
  /**
   * @param {!proto.proto.api.v1.BoardGPIOSetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardGPIOSetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardGPIOSetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardGPIOSetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardGPIOSetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardGPIOSet =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOSet,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardGPIOSetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardGPIOSetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardGPIOSet =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOSet);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardGPIOGetRequest,
 *   !proto.proto.api.v1.BoardGPIOGetResponse>}
 */
const methodDescriptor_RobotService_BoardGPIOGet = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardGPIOGet',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardGPIOGetRequest,
  proto.proto.api.v1.BoardGPIOGetResponse,
  /**
   * @param {!proto.proto.api.v1.BoardGPIOGetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardGPIOGetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardGPIOGetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardGPIOGetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardGPIOGetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardGPIOGet =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOGet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOGet,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardGPIOGetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardGPIOGetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardGPIOGet =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOGet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOGet);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardPWMSetRequest,
 *   !proto.proto.api.v1.BoardPWMSetResponse>}
 */
const methodDescriptor_RobotService_BoardPWMSet = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardPWMSet',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardPWMSetRequest,
  proto.proto.api.v1.BoardPWMSetResponse,
  /**
   * @param {!proto.proto.api.v1.BoardPWMSetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardPWMSetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardPWMSetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardPWMSetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardPWMSetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardPWMSet =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSet,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardPWMSetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardPWMSetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardPWMSet =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSet);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardPWMSetFrequencyRequest,
 *   !proto.proto.api.v1.BoardPWMSetFrequencyResponse>}
 */
const methodDescriptor_RobotService_BoardPWMSetFrequency = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardPWMSetFrequency',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardPWMSetFrequencyRequest,
  proto.proto.api.v1.BoardPWMSetFrequencyResponse,
  /**
   * @param {!proto.proto.api.v1.BoardPWMSetFrequencyRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardPWMSetFrequencyResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardPWMSetFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardPWMSetFrequencyResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardPWMSetFrequencyResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardPWMSetFrequency =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSetFrequency',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSetFrequency,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardPWMSetFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardPWMSetFrequencyResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardPWMSetFrequency =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSetFrequency',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSetFrequency);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardAnalogReaderReadRequest,
 *   !proto.proto.api.v1.BoardAnalogReaderReadResponse>}
 */
const methodDescriptor_RobotService_BoardAnalogReaderRead = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardAnalogReaderRead',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardAnalogReaderReadRequest,
  proto.proto.api.v1.BoardAnalogReaderReadResponse,
  /**
   * @param {!proto.proto.api.v1.BoardAnalogReaderReadRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardAnalogReaderReadResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardAnalogReaderReadRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardAnalogReaderReadResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardAnalogReaderReadResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardAnalogReaderRead =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardAnalogReaderRead',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardAnalogReaderRead,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardAnalogReaderReadRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardAnalogReaderReadResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardAnalogReaderRead =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardAnalogReaderRead',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardAnalogReaderRead);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardDigitalInterruptConfigRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptConfigResponse>}
 */
const methodDescriptor_RobotService_BoardDigitalInterruptConfig = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardDigitalInterruptConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardDigitalInterruptConfigRequest,
  proto.proto.api.v1.BoardDigitalInterruptConfigResponse,
  /**
   * @param {!proto.proto.api.v1.BoardDigitalInterruptConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardDigitalInterruptConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardDigitalInterruptConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardDigitalInterruptConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardDigitalInterruptConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptConfig,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardDigitalInterruptConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardDigitalInterruptConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardDigitalInterruptValueRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptValueResponse>}
 */
const methodDescriptor_RobotService_BoardDigitalInterruptValue = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardDigitalInterruptValue',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardDigitalInterruptValueRequest,
  proto.proto.api.v1.BoardDigitalInterruptValueResponse,
  /**
   * @param {!proto.proto.api.v1.BoardDigitalInterruptValueRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardDigitalInterruptValueResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardDigitalInterruptValueResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardDigitalInterruptValueResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardDigitalInterruptValue =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptValue,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardDigitalInterruptValueResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardDigitalInterruptValue =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptValue);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardDigitalInterruptTickRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptTickResponse>}
 */
const methodDescriptor_RobotService_BoardDigitalInterruptTick = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardDigitalInterruptTick',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardDigitalInterruptTickRequest,
  proto.proto.api.v1.BoardDigitalInterruptTickResponse,
  /**
   * @param {!proto.proto.api.v1.BoardDigitalInterruptTickRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardDigitalInterruptTickResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptTickRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardDigitalInterruptTickResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardDigitalInterruptTickResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardDigitalInterruptTick =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptTick',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptTick,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptTickRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardDigitalInterruptTickResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardDigitalInterruptTick =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptTick',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptTick);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.SensorReadingsRequest,
 *   !proto.proto.api.v1.SensorReadingsResponse>}
 */
const methodDescriptor_RobotService_SensorReadings = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/SensorReadings',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.SensorReadingsRequest,
  proto.proto.api.v1.SensorReadingsResponse,
  /**
   * @param {!proto.proto.api.v1.SensorReadingsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.SensorReadingsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.SensorReadingsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.SensorReadingsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.SensorReadingsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.sensorReadings =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/SensorReadings',
      request,
      metadata || {},
      methodDescriptor_RobotService_SensorReadings,
      callback);
};


/**
 * @param {!proto.proto.api.v1.SensorReadingsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.SensorReadingsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.sensorReadings =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/SensorReadings',
      request,
      metadata || {},
      methodDescriptor_RobotService_SensorReadings);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassHeadingRequest,
 *   !proto.proto.api.v1.CompassHeadingResponse>}
 */
const methodDescriptor_RobotService_CompassHeading = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassHeading',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassHeadingRequest,
  proto.proto.api.v1.CompassHeadingResponse,
  /**
   * @param {!proto.proto.api.v1.CompassHeadingRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassHeadingResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassHeadingRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassHeadingResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassHeadingResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassHeading =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassHeading',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassHeading,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassHeadingRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassHeadingResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassHeading =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassHeading',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassHeading);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassStartCalibrationRequest,
 *   !proto.proto.api.v1.CompassStartCalibrationResponse>}
 */
const methodDescriptor_RobotService_CompassStartCalibration = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassStartCalibration',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassStartCalibrationRequest,
  proto.proto.api.v1.CompassStartCalibrationResponse,
  /**
   * @param {!proto.proto.api.v1.CompassStartCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassStartCalibrationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassStartCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassStartCalibrationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassStartCalibrationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassStartCalibration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStartCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStartCalibration,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassStartCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassStartCalibrationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassStartCalibration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStartCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStartCalibration);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassStopCalibrationRequest,
 *   !proto.proto.api.v1.CompassStopCalibrationResponse>}
 */
const methodDescriptor_RobotService_CompassStopCalibration = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassStopCalibration',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassStopCalibrationRequest,
  proto.proto.api.v1.CompassStopCalibrationResponse,
  /**
   * @param {!proto.proto.api.v1.CompassStopCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassStopCalibrationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassStopCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassStopCalibrationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassStopCalibrationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassStopCalibration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStopCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStopCalibration,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassStopCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassStopCalibrationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassStopCalibration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStopCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStopCalibration);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassMarkRequest,
 *   !proto.proto.api.v1.CompassMarkResponse>}
 */
const methodDescriptor_RobotService_CompassMark = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassMark',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassMarkRequest,
  proto.proto.api.v1.CompassMarkResponse,
  /**
   * @param {!proto.proto.api.v1.CompassMarkRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassMarkResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassMarkRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassMarkResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassMarkResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassMark =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassMark',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassMark,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassMarkRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassMarkResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassMark =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassMark',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassMark);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ForceMatrixMatrixRequest,
 *   !proto.proto.api.v1.ForceMatrixMatrixResponse>}
 */
const methodDescriptor_RobotService_ForceMatrixMatrix = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ForceMatrixMatrix',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ForceMatrixMatrixRequest,
  proto.proto.api.v1.ForceMatrixMatrixResponse,
  /**
   * @param {!proto.proto.api.v1.ForceMatrixMatrixRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ForceMatrixMatrixResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ForceMatrixMatrixRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ForceMatrixMatrixResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ForceMatrixMatrixResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.forceMatrixMatrix =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixMatrix',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixMatrix,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ForceMatrixMatrixRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ForceMatrixMatrixResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.forceMatrixMatrix =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixMatrix',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixMatrix);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ForceMatrixSlipDetectionRequest,
 *   !proto.proto.api.v1.ForceMatrixSlipDetectionResponse>}
 */
const methodDescriptor_RobotService_ForceMatrixSlipDetection = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ForceMatrixSlipDetection',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ForceMatrixSlipDetectionRequest,
  proto.proto.api.v1.ForceMatrixSlipDetectionResponse,
  /**
   * @param {!proto.proto.api.v1.ForceMatrixSlipDetectionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ForceMatrixSlipDetectionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ForceMatrixSlipDetectionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ForceMatrixSlipDetectionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ForceMatrixSlipDetectionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.forceMatrixSlipDetection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixSlipDetection',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixSlipDetection,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ForceMatrixSlipDetectionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ForceMatrixSlipDetectionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.forceMatrixSlipDetection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixSlipDetection',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixSlipDetection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ExecuteFunctionRequest,
 *   !proto.proto.api.v1.ExecuteFunctionResponse>}
 */
const methodDescriptor_RobotService_ExecuteFunction = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ExecuteFunction',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ExecuteFunctionRequest,
  proto.proto.api.v1.ExecuteFunctionResponse,
  /**
   * @param {!proto.proto.api.v1.ExecuteFunctionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ExecuteFunctionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ExecuteFunctionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ExecuteFunctionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ExecuteFunctionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.executeFunction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteFunction',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteFunction,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ExecuteFunctionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ExecuteFunctionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.executeFunction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteFunction',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteFunction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ExecuteSourceRequest,
 *   !proto.proto.api.v1.ExecuteSourceResponse>}
 */
const methodDescriptor_RobotService_ExecuteSource = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ExecuteSource',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ExecuteSourceRequest,
  proto.proto.api.v1.ExecuteSourceResponse,
  /**
   * @param {!proto.proto.api.v1.ExecuteSourceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ExecuteSourceResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ExecuteSourceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ExecuteSourceResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ExecuteSourceResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.executeSource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteSource',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteSource,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ExecuteSourceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ExecuteSourceResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.executeSource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteSource',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteSource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.InputControllerControlsRequest,
 *   !proto.proto.api.v1.InputControllerControlsResponse>}
 */
const methodDescriptor_RobotService_InputControllerControls = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/InputControllerControls',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.InputControllerControlsRequest,
  proto.proto.api.v1.InputControllerControlsResponse,
  /**
   * @param {!proto.proto.api.v1.InputControllerControlsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.InputControllerControlsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.InputControllerControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.InputControllerControlsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerControlsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.inputControllerControls =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerControls',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerControls,
      callback);
};


/**
 * @param {!proto.proto.api.v1.InputControllerControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.InputControllerControlsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.inputControllerControls =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerControls',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerControls);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.InputControllerLastEventsRequest,
 *   !proto.proto.api.v1.InputControllerLastEventsResponse>}
 */
const methodDescriptor_RobotService_InputControllerLastEvents = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/InputControllerLastEvents',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.InputControllerLastEventsRequest,
  proto.proto.api.v1.InputControllerLastEventsResponse,
  /**
   * @param {!proto.proto.api.v1.InputControllerLastEventsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.InputControllerLastEventsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.InputControllerLastEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.InputControllerLastEventsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerLastEventsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.inputControllerLastEvents =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerLastEvents',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerLastEvents,
      callback);
};


/**
 * @param {!proto.proto.api.v1.InputControllerLastEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.InputControllerLastEventsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.inputControllerLastEvents =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerLastEvents',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerLastEvents);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.InputControllerEventStreamRequest,
 *   !proto.proto.api.v1.InputControllerEvent>}
 */
const methodDescriptor_RobotService_InputControllerEventStream = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/InputControllerEventStream',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.proto.api.v1.InputControllerEventStreamRequest,
  proto.proto.api.v1.InputControllerEvent,
  /**
   * @param {!proto.proto.api.v1.InputControllerEventStreamRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.InputControllerEvent.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.InputControllerEventStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerEvent>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.inputControllerEventStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerEventStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerEventStream);
};


/**
 * @param {!proto.proto.api.v1.InputControllerEventStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerEvent>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.inputControllerEventStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerEventStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerEventStream);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.InputControllerInjectEventRequest,
 *   !proto.proto.api.v1.InputControllerInjectEventResponse>}
 */
const methodDescriptor_RobotService_InputControllerInjectEvent = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/InputControllerInjectEvent',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.InputControllerInjectEventRequest,
  proto.proto.api.v1.InputControllerInjectEventResponse,
  /**
   * @param {!proto.proto.api.v1.InputControllerInjectEventRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.InputControllerInjectEventResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.InputControllerInjectEventRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.InputControllerInjectEventResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerInjectEventResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.inputControllerInjectEvent =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerInjectEvent',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerInjectEvent,
      callback);
};


/**
 * @param {!proto.proto.api.v1.InputControllerInjectEventRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.InputControllerInjectEventResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.inputControllerInjectEvent =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerInjectEvent',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerInjectEvent);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ResourceRunCommandRequest,
 *   !proto.proto.api.v1.ResourceRunCommandResponse>}
 */
const methodDescriptor_RobotService_ResourceRunCommand = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ResourceRunCommand',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ResourceRunCommandRequest,
  proto.proto.api.v1.ResourceRunCommandResponse,
  /**
   * @param {!proto.proto.api.v1.ResourceRunCommandRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ResourceRunCommandResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ResourceRunCommandRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ResourceRunCommandResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ResourceRunCommandResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.resourceRunCommand =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ResourceRunCommand',
      request,
      metadata || {},
      methodDescriptor_RobotService_ResourceRunCommand,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ResourceRunCommandRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ResourceRunCommandResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.resourceRunCommand =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ResourceRunCommand',
      request,
      metadata || {},
      methodDescriptor_RobotService_ResourceRunCommand);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.FrameServiceConfigRequest,
 *   !proto.proto.api.v1.FrameServiceConfigResponse>}
 */
const methodDescriptor_RobotService_FrameServiceConfig = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/FrameServiceConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.FrameServiceConfigRequest,
  proto.proto.api.v1.FrameServiceConfigResponse,
  /**
   * @param {!proto.proto.api.v1.FrameServiceConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.FrameServiceConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.FrameServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.FrameServiceConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.FrameServiceConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.frameServiceConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/FrameServiceConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_FrameServiceConfig,
      callback);
};


/**
 * @param {!proto.proto.api.v1.FrameServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.FrameServiceConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.frameServiceConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/FrameServiceConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_FrameServiceConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceModeRequest,
 *   !proto.proto.api.v1.NavigationServiceModeResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceMode = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceMode',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceModeRequest,
  proto.proto.api.v1.NavigationServiceModeResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceModeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceModeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceModeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceMode =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceMode,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceModeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceModeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceMode =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceMode);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceSetModeRequest,
 *   !proto.proto.api.v1.NavigationServiceSetModeResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceSetMode = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceSetMode',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceSetModeRequest,
  proto.proto.api.v1.NavigationServiceSetModeResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceSetModeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceSetModeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceSetModeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceSetMode =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceSetMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceSetMode,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceSetModeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceSetMode =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceSetMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceSetMode);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceLocationRequest,
 *   !proto.proto.api.v1.NavigationServiceLocationResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceLocation = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceLocation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceLocationRequest,
  proto.proto.api.v1.NavigationServiceLocationResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceLocationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceLocationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceLocationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceLocation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceLocation,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceLocationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceLocation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceLocation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceWaypointsRequest,
 *   !proto.proto.api.v1.NavigationServiceWaypointsResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceWaypoints = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceWaypoints',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceWaypointsRequest,
  proto.proto.api.v1.NavigationServiceWaypointsResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceWaypointsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceWaypointsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceWaypointsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceWaypoints =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceWaypoints',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceWaypoints,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceWaypointsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceWaypoints =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceWaypoints',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceWaypoints);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceAddWaypointRequest,
 *   !proto.proto.api.v1.NavigationServiceAddWaypointResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceAddWaypoint = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceAddWaypoint',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceAddWaypointRequest,
  proto.proto.api.v1.NavigationServiceAddWaypointResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceAddWaypointResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceAddWaypointResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceAddWaypointResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceAddWaypoint =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceAddWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceAddWaypoint,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceAddWaypointResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceAddWaypoint =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceAddWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceAddWaypoint);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceRemoveWaypointRequest,
 *   !proto.proto.api.v1.NavigationServiceRemoveWaypointResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceRemoveWaypoint = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceRemoveWaypoint',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceRemoveWaypointRequest,
  proto.proto.api.v1.NavigationServiceRemoveWaypointResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceRemoveWaypointResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceRemoveWaypoint =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceRemoveWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceRemoveWaypoint,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceRemoveWaypoint =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceRemoveWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceRemoveWaypoint);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest,
 *   !proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse>}
 */
const methodDescriptor_RobotService_ObjectManipulationServiceDoGrab = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ObjectManipulationServiceDoGrab',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest,
  proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse,
  /**
   * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.objectManipulationServiceDoGrab =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ObjectManipulationServiceDoGrab',
      request,
      metadata || {},
      methodDescriptor_RobotService_ObjectManipulationServiceDoGrab,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.objectManipulationServiceDoGrab =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ObjectManipulationServiceDoGrab',
      request,
      metadata || {},
      methodDescriptor_RobotService_ObjectManipulationServiceDoGrab);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSLocationRequest,
 *   !proto.proto.api.v1.GPSLocationResponse>}
 */
const methodDescriptor_RobotService_GPSLocation = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSLocation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSLocationRequest,
  proto.proto.api.v1.GPSLocationResponse,
  /**
   * @param {!proto.proto.api.v1.GPSLocationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSLocationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSLocationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSLocationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSLocation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSLocation,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSLocationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSLocation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSLocation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSAltitudeRequest,
 *   !proto.proto.api.v1.GPSAltitudeResponse>}
 */
const methodDescriptor_RobotService_GPSAltitude = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSAltitude',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSAltitudeRequest,
  proto.proto.api.v1.GPSAltitudeResponse,
  /**
   * @param {!proto.proto.api.v1.GPSAltitudeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSAltitudeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSAltitudeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSAltitudeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSAltitudeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSAltitude =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAltitude',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAltitude,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSAltitudeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSAltitudeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSAltitude =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAltitude',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAltitude);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSSpeedRequest,
 *   !proto.proto.api.v1.GPSSpeedResponse>}
 */
const methodDescriptor_RobotService_GPSSpeed = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSSpeed',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSSpeedRequest,
  proto.proto.api.v1.GPSSpeedResponse,
  /**
   * @param {!proto.proto.api.v1.GPSSpeedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSSpeedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSSpeedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSSpeedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSSpeedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSSpeed =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSSpeed',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSSpeed,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSSpeedRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSSpeedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSSpeed =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSSpeed',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSSpeed);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSAccuracyRequest,
 *   !proto.proto.api.v1.GPSAccuracyResponse>}
 */
const methodDescriptor_RobotService_GPSAccuracy = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSAccuracy',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSAccuracyRequest,
  proto.proto.api.v1.GPSAccuracyResponse,
  /**
   * @param {!proto.proto.api.v1.GPSAccuracyRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSAccuracyResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSAccuracyRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSAccuracyResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSAccuracyResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSAccuracy =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAccuracy',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAccuracy,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSAccuracyRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSAccuracyResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSAccuracy =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAccuracy',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAccuracy);
};


module.exports = proto.proto.api.v1;

