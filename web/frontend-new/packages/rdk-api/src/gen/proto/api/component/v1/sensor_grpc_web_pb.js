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
proto.proto.api.component.v1 = require('./sensor_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.SensorServiceClient =
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
proto.proto.api.component.v1.SensorServicePromiseClient =
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
 *   !proto.proto.api.component.v1.SensorServiceGetReadingsRequest,
 *   !proto.proto.api.component.v1.SensorServiceGetReadingsResponse>}
 */
const methodDescriptor_SensorService_GetReadings = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.SensorService/GetReadings',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.SensorServiceGetReadingsRequest,
  proto.proto.api.component.v1.SensorServiceGetReadingsResponse,
  /**
   * @param {!proto.proto.api.component.v1.SensorServiceGetReadingsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.SensorServiceGetReadingsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.SensorServiceGetReadingsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.SensorServiceGetReadingsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.SensorServiceGetReadingsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.SensorServiceClient.prototype.getReadings =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.SensorService/GetReadings',
      request,
      metadata || {},
      methodDescriptor_SensorService_GetReadings,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.SensorServiceGetReadingsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.SensorServiceGetReadingsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.SensorServicePromiseClient.prototype.getReadings =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.SensorService/GetReadings',
      request,
      metadata || {},
      methodDescriptor_SensorService_GetReadings);
};


module.exports = proto.proto.api.component.v1;

