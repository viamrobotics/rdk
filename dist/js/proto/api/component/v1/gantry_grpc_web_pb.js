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
 *   !proto.proto.api.component.v1.GantryServiceCurrentPositionRequest,
 *   !proto.proto.api.component.v1.GantryServiceCurrentPositionResponse>}
 */
const methodDescriptor_GantryService_CurrentPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GantryService/CurrentPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GantryServiceCurrentPositionRequest,
  proto.proto.api.component.v1.GantryServiceCurrentPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.GantryServiceCurrentPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GantryServiceCurrentPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GantryServiceCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GantryServiceCurrentPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GantryServiceCurrentPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GantryServiceClient.prototype.currentPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/CurrentPosition',
      request,
      metadata || {},
      methodDescriptor_GantryService_CurrentPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GantryServiceCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GantryServiceCurrentPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GantryServicePromiseClient.prototype.currentPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/CurrentPosition',
      request,
      metadata || {},
      methodDescriptor_GantryService_CurrentPosition);
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
 *   !proto.proto.api.component.v1.GantryServiceLengthsRequest,
 *   !proto.proto.api.component.v1.GantryServiceLengthsResponse>}
 */
const methodDescriptor_GantryService_Lengths = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GantryService/Lengths',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GantryServiceLengthsRequest,
  proto.proto.api.component.v1.GantryServiceLengthsResponse,
  /**
   * @param {!proto.proto.api.component.v1.GantryServiceLengthsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GantryServiceLengthsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GantryServiceLengthsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GantryServiceLengthsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GantryServiceLengthsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GantryServiceClient.prototype.lengths =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/Lengths',
      request,
      metadata || {},
      methodDescriptor_GantryService_Lengths,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GantryServiceLengthsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GantryServiceLengthsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GantryServicePromiseClient.prototype.lengths =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GantryService/Lengths',
      request,
      metadata || {},
      methodDescriptor_GantryService_Lengths);
};


module.exports = proto.proto.api.component.v1;

