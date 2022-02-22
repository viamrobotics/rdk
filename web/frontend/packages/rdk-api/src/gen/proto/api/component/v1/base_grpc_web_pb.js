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
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.component = {};
proto.proto.api.component.v1 = require('./base_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.BaseServiceClient =
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
proto.proto.api.component.v1.BaseServicePromiseClient =
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
 *   !proto.proto.api.component.v1.BaseServiceMoveStraightRequest,
 *   !proto.proto.api.component.v1.BaseServiceMoveStraightResponse>}
 */
const methodDescriptor_BaseService_MoveStraight = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BaseService/MoveStraight',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BaseServiceMoveStraightRequest,
  proto.proto.api.component.v1.BaseServiceMoveStraightResponse,
  /**
   * @param {!proto.proto.api.component.v1.BaseServiceMoveStraightRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BaseServiceMoveStraightResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BaseServiceMoveStraightRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BaseServiceMoveStraightResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BaseServiceMoveStraightResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BaseServiceClient.prototype.moveStraight =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/MoveStraight',
      request,
      metadata || {},
      methodDescriptor_BaseService_MoveStraight,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BaseServiceMoveStraightRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BaseServiceMoveStraightResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BaseServicePromiseClient.prototype.moveStraight =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/MoveStraight',
      request,
      metadata || {},
      methodDescriptor_BaseService_MoveStraight);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BaseServiceMoveArcRequest,
 *   !proto.proto.api.component.v1.BaseServiceMoveArcResponse>}
 */
const methodDescriptor_BaseService_MoveArc = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BaseService/MoveArc',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BaseServiceMoveArcRequest,
  proto.proto.api.component.v1.BaseServiceMoveArcResponse,
  /**
   * @param {!proto.proto.api.component.v1.BaseServiceMoveArcRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BaseServiceMoveArcResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BaseServiceMoveArcRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BaseServiceMoveArcResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BaseServiceMoveArcResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BaseServiceClient.prototype.moveArc =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/MoveArc',
      request,
      metadata || {},
      methodDescriptor_BaseService_MoveArc,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BaseServiceMoveArcRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BaseServiceMoveArcResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BaseServicePromiseClient.prototype.moveArc =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/MoveArc',
      request,
      metadata || {},
      methodDescriptor_BaseService_MoveArc);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BaseServiceSpinRequest,
 *   !proto.proto.api.component.v1.BaseServiceSpinResponse>}
 */
const methodDescriptor_BaseService_Spin = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BaseService/Spin',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BaseServiceSpinRequest,
  proto.proto.api.component.v1.BaseServiceSpinResponse,
  /**
   * @param {!proto.proto.api.component.v1.BaseServiceSpinRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BaseServiceSpinResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BaseServiceSpinRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BaseServiceSpinResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BaseServiceSpinResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BaseServiceClient.prototype.spin =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/Spin',
      request,
      metadata || {},
      methodDescriptor_BaseService_Spin,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BaseServiceSpinRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BaseServiceSpinResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BaseServicePromiseClient.prototype.spin =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/Spin',
      request,
      metadata || {},
      methodDescriptor_BaseService_Spin);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.BaseServiceStopRequest,
 *   !proto.proto.api.component.v1.BaseServiceStopResponse>}
 */
const methodDescriptor_BaseService_Stop = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.BaseService/Stop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.BaseServiceStopRequest,
  proto.proto.api.component.v1.BaseServiceStopResponse,
  /**
   * @param {!proto.proto.api.component.v1.BaseServiceStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.BaseServiceStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.BaseServiceStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.BaseServiceStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.BaseServiceStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.BaseServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/Stop',
      request,
      metadata || {},
      methodDescriptor_BaseService_Stop,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.BaseServiceStopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.BaseServiceStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.BaseServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.BaseService/Stop',
      request,
      metadata || {},
      methodDescriptor_BaseService_Stop);
};


module.exports = proto.proto.api.component.v1;

