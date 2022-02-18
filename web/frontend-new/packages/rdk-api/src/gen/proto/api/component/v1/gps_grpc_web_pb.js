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
proto.proto.api.component.v1 = require('./gps_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.GPSServiceClient =
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
proto.proto.api.component.v1.GPSServicePromiseClient =
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
 *   !proto.proto.api.component.v1.GPSServiceReadLocationRequest,
 *   !proto.proto.api.component.v1.GPSServiceReadLocationResponse>}
 */
const methodDescriptor_GPSService_ReadLocation = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GPSService/ReadLocation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GPSServiceReadLocationRequest,
  proto.proto.api.component.v1.GPSServiceReadLocationResponse,
  /**
   * @param {!proto.proto.api.component.v1.GPSServiceReadLocationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GPSServiceReadLocationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GPSServiceReadLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GPSServiceReadLocationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GPSServiceReadLocationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GPSServiceClient.prototype.readLocation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GPSService/ReadLocation',
      request,
      metadata || {},
      methodDescriptor_GPSService_ReadLocation,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GPSServiceReadLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GPSServiceReadLocationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GPSServicePromiseClient.prototype.readLocation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GPSService/ReadLocation',
      request,
      metadata || {},
      methodDescriptor_GPSService_ReadLocation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.GPSServiceReadAltitudeRequest,
 *   !proto.proto.api.component.v1.GPSServiceReadAltitudeResponse>}
 */
const methodDescriptor_GPSService_ReadAltitude = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GPSService/ReadAltitude',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GPSServiceReadAltitudeRequest,
  proto.proto.api.component.v1.GPSServiceReadAltitudeResponse,
  /**
   * @param {!proto.proto.api.component.v1.GPSServiceReadAltitudeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GPSServiceReadAltitudeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GPSServiceReadAltitudeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GPSServiceReadAltitudeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GPSServiceReadAltitudeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GPSServiceClient.prototype.readAltitude =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GPSService/ReadAltitude',
      request,
      metadata || {},
      methodDescriptor_GPSService_ReadAltitude,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GPSServiceReadAltitudeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GPSServiceReadAltitudeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GPSServicePromiseClient.prototype.readAltitude =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GPSService/ReadAltitude',
      request,
      metadata || {},
      methodDescriptor_GPSService_ReadAltitude);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.GPSServiceReadSpeedRequest,
 *   !proto.proto.api.component.v1.GPSServiceReadSpeedResponse>}
 */
const methodDescriptor_GPSService_ReadSpeed = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GPSService/ReadSpeed',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GPSServiceReadSpeedRequest,
  proto.proto.api.component.v1.GPSServiceReadSpeedResponse,
  /**
   * @param {!proto.proto.api.component.v1.GPSServiceReadSpeedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GPSServiceReadSpeedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GPSServiceReadSpeedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GPSServiceReadSpeedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GPSServiceReadSpeedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GPSServiceClient.prototype.readSpeed =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GPSService/ReadSpeed',
      request,
      metadata || {},
      methodDescriptor_GPSService_ReadSpeed,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GPSServiceReadSpeedRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GPSServiceReadSpeedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GPSServicePromiseClient.prototype.readSpeed =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GPSService/ReadSpeed',
      request,
      metadata || {},
      methodDescriptor_GPSService_ReadSpeed);
};


module.exports = proto.proto.api.component.v1;

