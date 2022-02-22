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
proto.proto.api.component.v1 = require('./imu_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.IMUServiceClient =
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
proto.proto.api.component.v1.IMUServicePromiseClient =
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
 *   !proto.proto.api.component.v1.IMUServiceReadAngularVelocityRequest,
 *   !proto.proto.api.component.v1.IMUServiceReadAngularVelocityResponse>}
 */
const methodDescriptor_IMUService_ReadAngularVelocity = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.IMUService/ReadAngularVelocity',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.IMUServiceReadAngularVelocityRequest,
  proto.proto.api.component.v1.IMUServiceReadAngularVelocityResponse,
  /**
   * @param {!proto.proto.api.component.v1.IMUServiceReadAngularVelocityRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.IMUServiceReadAngularVelocityResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.IMUServiceReadAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.IMUServiceReadAngularVelocityResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.IMUServiceReadAngularVelocityResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.IMUServiceClient.prototype.readAngularVelocity =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/ReadAngularVelocity',
      request,
      metadata || {},
      methodDescriptor_IMUService_ReadAngularVelocity,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.IMUServiceReadAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.IMUServiceReadAngularVelocityResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.IMUServicePromiseClient.prototype.readAngularVelocity =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/ReadAngularVelocity',
      request,
      metadata || {},
      methodDescriptor_IMUService_ReadAngularVelocity);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.IMUServiceReadOrientationRequest,
 *   !proto.proto.api.component.v1.IMUServiceReadOrientationResponse>}
 */
const methodDescriptor_IMUService_ReadOrientation = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.IMUService/ReadOrientation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.IMUServiceReadOrientationRequest,
  proto.proto.api.component.v1.IMUServiceReadOrientationResponse,
  /**
   * @param {!proto.proto.api.component.v1.IMUServiceReadOrientationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.IMUServiceReadOrientationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.IMUServiceReadOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.IMUServiceReadOrientationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.IMUServiceReadOrientationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.IMUServiceClient.prototype.readOrientation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/ReadOrientation',
      request,
      metadata || {},
      methodDescriptor_IMUService_ReadOrientation,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.IMUServiceReadOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.IMUServiceReadOrientationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.IMUServicePromiseClient.prototype.readOrientation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/ReadOrientation',
      request,
      metadata || {},
      methodDescriptor_IMUService_ReadOrientation);
};


module.exports = proto.proto.api.component.v1;

