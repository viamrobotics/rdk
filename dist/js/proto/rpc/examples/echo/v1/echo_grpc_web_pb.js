/**
 * @fileoverview gRPC-Web generated client stub for proto.rpc.examples.echo.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_annotations_pb = require('../../../../../google/api/annotations_pb.js')
const proto = {};
proto.proto = {};
proto.proto.rpc = {};
proto.proto.rpc.examples = {};
proto.proto.rpc.examples.echo = {};
proto.proto.rpc.examples.echo.v1 = require('./echo_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.rpc.examples.echo.v1.EchoServiceClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options['format'] = 'text';

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
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.rpc.examples.echo.v1.EchoServicePromiseClient =
    function(hostname, credentials, options) {
  if (!options) options = {};
  options['format'] = 'text';

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
 *   !proto.proto.rpc.examples.echo.v1.EchoRequest,
 *   !proto.proto.rpc.examples.echo.v1.EchoResponse>}
 */
const methodDescriptor_EchoService_Echo = new grpc.web.MethodDescriptor(
  '/proto.rpc.examples.echo.v1.EchoService/Echo',
  grpc.web.MethodType.UNARY,
  proto.proto.rpc.examples.echo.v1.EchoRequest,
  proto.proto.rpc.examples.echo.v1.EchoResponse,
  /**
   * @param {!proto.proto.rpc.examples.echo.v1.EchoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.rpc.examples.echo.v1.EchoResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.rpc.examples.echo.v1.EchoRequest,
 *   !proto.proto.rpc.examples.echo.v1.EchoResponse>}
 */
const methodInfo_EchoService_Echo = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.rpc.examples.echo.v1.EchoResponse,
  /**
   * @param {!proto.proto.rpc.examples.echo.v1.EchoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.rpc.examples.echo.v1.EchoResponse.deserializeBinary
);


/**
 * @param {!proto.proto.rpc.examples.echo.v1.EchoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.rpc.examples.echo.v1.EchoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.rpc.examples.echo.v1.EchoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.rpc.examples.echo.v1.EchoServiceClient.prototype.echo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.rpc.examples.echo.v1.EchoService/Echo',
      request,
      metadata || {},
      methodDescriptor_EchoService_Echo,
      callback);
};


/**
 * @param {!proto.proto.rpc.examples.echo.v1.EchoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.rpc.examples.echo.v1.EchoResponse>}
 *     Promise that resolves to the response
 */
proto.proto.rpc.examples.echo.v1.EchoServicePromiseClient.prototype.echo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.rpc.examples.echo.v1.EchoService/Echo',
      request,
      metadata || {},
      methodDescriptor_EchoService_Echo);
};


module.exports = proto.proto.rpc.examples.echo.v1;

