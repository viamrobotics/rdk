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
 *   !proto.proto.api.component.v1.IMUServiceAngularVelocityRequest,
 *   !proto.proto.api.component.v1.IMUServiceAngularVelocityResponse>}
 */
const methodDescriptor_IMUService_AngularVelocity = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.IMUService/AngularVelocity',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.IMUServiceAngularVelocityRequest,
  proto.proto.api.component.v1.IMUServiceAngularVelocityResponse,
  /**
   * @param {!proto.proto.api.component.v1.IMUServiceAngularVelocityRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.IMUServiceAngularVelocityResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.IMUServiceAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.IMUServiceAngularVelocityResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.IMUServiceAngularVelocityResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.IMUServiceClient.prototype.angularVelocity =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/AngularVelocity',
      request,
      metadata || {},
      methodDescriptor_IMUService_AngularVelocity,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.IMUServiceAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.IMUServiceAngularVelocityResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.IMUServicePromiseClient.prototype.angularVelocity =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/AngularVelocity',
      request,
      metadata || {},
      methodDescriptor_IMUService_AngularVelocity);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.IMUServiceOrientationRequest,
 *   !proto.proto.api.component.v1.IMUServiceOrientationResponse>}
 */
const methodDescriptor_IMUService_Orientation = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.IMUService/Orientation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.IMUServiceOrientationRequest,
  proto.proto.api.component.v1.IMUServiceOrientationResponse,
  /**
   * @param {!proto.proto.api.component.v1.IMUServiceOrientationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.IMUServiceOrientationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.IMUServiceOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.IMUServiceOrientationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.IMUServiceOrientationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.IMUServiceClient.prototype.orientation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/Orientation',
      request,
      metadata || {},
      methodDescriptor_IMUService_Orientation,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.IMUServiceOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.IMUServiceOrientationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.IMUServicePromiseClient.prototype.orientation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/Orientation',
      request,
      metadata || {},
      methodDescriptor_IMUService_Orientation);
};


module.exports = proto.proto.api.component.v1;

