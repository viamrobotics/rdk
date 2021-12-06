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
proto.proto.api.component.v1 = require('./servo_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.ServoServiceClient =
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
proto.proto.api.component.v1.ServoServicePromiseClient =
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
 *   !proto.proto.api.component.v1.ServoServiceMoveRequest,
 *   !proto.proto.api.component.v1.ServoServiceMoveResponse>}
 */
const methodDescriptor_ServoService_Move = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ServoService/Move',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ServoServiceMoveRequest,
  proto.proto.api.component.v1.ServoServiceMoveResponse,
  /**
   * @param {!proto.proto.api.component.v1.ServoServiceMoveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ServoServiceMoveResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ServoServiceMoveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ServoServiceMoveResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ServoServiceMoveResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ServoServiceClient.prototype.move =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ServoService/Move',
      request,
      metadata || {},
      methodDescriptor_ServoService_Move,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ServoServiceMoveRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ServoServiceMoveResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ServoServicePromiseClient.prototype.move =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ServoService/Move',
      request,
      metadata || {},
      methodDescriptor_ServoService_Move);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ServoServiceAngularOffsetRequest,
 *   !proto.proto.api.component.v1.ServoServiceAngularOffsetResponse>}
 */
const methodDescriptor_ServoService_AngularOffset = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ServoService/AngularOffset',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ServoServiceAngularOffsetRequest,
  proto.proto.api.component.v1.ServoServiceAngularOffsetResponse,
  /**
   * @param {!proto.proto.api.component.v1.ServoServiceAngularOffsetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ServoServiceAngularOffsetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ServoServiceAngularOffsetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ServoServiceAngularOffsetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ServoServiceAngularOffsetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ServoServiceClient.prototype.angularOffset =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ServoService/AngularOffset',
      request,
      metadata || {},
      methodDescriptor_ServoService_AngularOffset,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ServoServiceAngularOffsetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ServoServiceAngularOffsetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ServoServicePromiseClient.prototype.angularOffset =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ServoService/AngularOffset',
      request,
      metadata || {},
      methodDescriptor_ServoService_AngularOffset);
};


module.exports = proto.proto.api.component.v1;

