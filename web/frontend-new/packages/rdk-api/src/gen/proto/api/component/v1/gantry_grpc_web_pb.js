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
proto.proto.api.component.v1 = require('./gantry_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.GantryServiceClient =
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
proto.proto.api.component.v1.GantryServicePromiseClient =
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
 *   !proto.proto.api.component.v1.GantryServiceGetPositionRequest,
 *   !proto.proto.api.component.v1.GantryServiceGetPositionResponse>}
 */
const methodDescriptor_GantryService_GetPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GantryService/GetPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GantryServiceGetPositionRequest,
  proto.proto.api.component.v1.GantryServiceGetPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.GantryServiceGetPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GantryServiceGetPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GantryServiceGetPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GantryServiceGetPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GantryServiceGetPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GantryServiceClient.prototype.getPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/GetPosition',
      request,
      metadata || {},
      methodDescriptor_GantryService_GetPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GantryServiceGetPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GantryServiceGetPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GantryServicePromiseClient.prototype.getPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/GetPosition',
      request,
      metadata || {},
      methodDescriptor_GantryService_GetPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.GantryServiceMoveToPositionRequest,
 *   !proto.proto.api.component.v1.GantryServiceMoveToPositionResponse>}
 */
const methodDescriptor_GantryService_MoveToPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GantryService/MoveToPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GantryServiceMoveToPositionRequest,
  proto.proto.api.component.v1.GantryServiceMoveToPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.GantryServiceMoveToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GantryServiceMoveToPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GantryServiceMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GantryServiceMoveToPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GantryServiceMoveToPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GantryServiceClient.prototype.moveToPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/MoveToPosition',
      request,
      metadata || {},
      methodDescriptor_GantryService_MoveToPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GantryServiceMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GantryServiceMoveToPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GantryServicePromiseClient.prototype.moveToPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/MoveToPosition',
      request,
      metadata || {},
      methodDescriptor_GantryService_MoveToPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.GantryServiceGetLengthsRequest,
 *   !proto.proto.api.component.v1.GantryServiceGetLengthsResponse>}
 */
const methodDescriptor_GantryService_GetLengths = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GantryService/GetLengths',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GantryServiceGetLengthsRequest,
  proto.proto.api.component.v1.GantryServiceGetLengthsResponse,
  /**
   * @param {!proto.proto.api.component.v1.GantryServiceGetLengthsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GantryServiceGetLengthsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GantryServiceGetLengthsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GantryServiceGetLengthsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GantryServiceGetLengthsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GantryServiceClient.prototype.getLengths =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/GetLengths',
      request,
      metadata || {},
      methodDescriptor_GantryService_GetLengths,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GantryServiceGetLengthsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GantryServiceGetLengthsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GantryServicePromiseClient.prototype.getLengths =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/GetLengths',
      request,
      metadata || {},
      methodDescriptor_GantryService_GetLengths);
};


module.exports = proto.proto.api.component.v1;

