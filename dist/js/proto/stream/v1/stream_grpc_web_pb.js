/**
 * @fileoverview gRPC-Web generated client stub for proto.stream.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_annotations_pb = require('../../../google/api/annotations_pb.js')
const proto = {};
proto.proto = {};
proto.proto.stream = {};
proto.proto.stream.v1 = require('./stream_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.stream.v1.StreamServiceClient =
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
proto.proto.stream.v1.StreamServicePromiseClient =
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
 *   !proto.proto.stream.v1.ListStreamsRequest,
 *   !proto.proto.stream.v1.ListStreamsResponse>}
 */
const methodDescriptor_StreamService_ListStreams = new grpc.web.MethodDescriptor(
  '/proto.stream.v1.StreamService/ListStreams',
  grpc.web.MethodType.UNARY,
  proto.proto.stream.v1.ListStreamsRequest,
  proto.proto.stream.v1.ListStreamsResponse,
  /**
   * @param {!proto.proto.stream.v1.ListStreamsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.stream.v1.ListStreamsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.stream.v1.ListStreamsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.stream.v1.ListStreamsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.stream.v1.ListStreamsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.stream.v1.StreamServiceClient.prototype.listStreams =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.stream.v1.StreamService/ListStreams',
      request,
      metadata || {},
      methodDescriptor_StreamService_ListStreams,
      callback);
};


/**
 * @param {!proto.proto.stream.v1.ListStreamsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.stream.v1.ListStreamsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.stream.v1.StreamServicePromiseClient.prototype.listStreams =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.stream.v1.StreamService/ListStreams',
      request,
      metadata || {},
      methodDescriptor_StreamService_ListStreams);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.stream.v1.AddStreamRequest,
 *   !proto.proto.stream.v1.AddStreamResponse>}
 */
const methodDescriptor_StreamService_AddStream = new grpc.web.MethodDescriptor(
  '/proto.stream.v1.StreamService/AddStream',
  grpc.web.MethodType.UNARY,
  proto.proto.stream.v1.AddStreamRequest,
  proto.proto.stream.v1.AddStreamResponse,
  /**
   * @param {!proto.proto.stream.v1.AddStreamRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.stream.v1.AddStreamResponse.deserializeBinary
);


/**
 * @param {!proto.proto.stream.v1.AddStreamRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.stream.v1.AddStreamResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.stream.v1.AddStreamResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.stream.v1.StreamServiceClient.prototype.addStream =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.stream.v1.StreamService/AddStream',
      request,
      metadata || {},
      methodDescriptor_StreamService_AddStream,
      callback);
};


/**
 * @param {!proto.proto.stream.v1.AddStreamRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.stream.v1.AddStreamResponse>}
 *     Promise that resolves to the response
 */
proto.proto.stream.v1.StreamServicePromiseClient.prototype.addStream =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.stream.v1.StreamService/AddStream',
      request,
      metadata || {},
      methodDescriptor_StreamService_AddStream);
};


module.exports = proto.proto.stream.v1;

