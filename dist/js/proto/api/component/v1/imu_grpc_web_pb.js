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

var google_protobuf_duration_pb = require('google-protobuf/google/protobuf/duration_pb.js')

var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js')

var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var google_api_httpbody_pb = require('../../../../google/api/httpbody_pb.js')

var proto_api_common_v1_common_pb = require('../../../../proto/api/common/v1/common_pb.js')
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
 *   !proto.proto.api.component.v1.IMUAngularVelocityRequest,
 *   !proto.proto.api.component.v1.IMUAngularVelocityResponse>}
 */
const methodDescriptor_IMUService_IMUAngularVelocity = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.IMUService/IMUAngularVelocity',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.IMUAngularVelocityRequest,
  proto.proto.api.component.v1.IMUAngularVelocityResponse,
  /**
   * @param {!proto.proto.api.component.v1.IMUAngularVelocityRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.IMUAngularVelocityResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.IMUAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.IMUAngularVelocityResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.IMUAngularVelocityResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.IMUServiceClient.prototype.iMUAngularVelocity =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/IMUAngularVelocity',
      request,
      metadata || {},
      methodDescriptor_IMUService_IMUAngularVelocity,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.IMUAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.IMUAngularVelocityResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.IMUServicePromiseClient.prototype.iMUAngularVelocity =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/IMUAngularVelocity',
      request,
      metadata || {},
      methodDescriptor_IMUService_IMUAngularVelocity);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.IMUOrientationRequest,
 *   !proto.proto.api.component.v1.IMUOrientationResponse>}
 */
const methodDescriptor_IMUService_IMUOrientation = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.IMUService/IMUOrientation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.IMUOrientationRequest,
  proto.proto.api.component.v1.IMUOrientationResponse,
  /**
   * @param {!proto.proto.api.component.v1.IMUOrientationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.IMUOrientationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.IMUOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.IMUOrientationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.IMUOrientationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.IMUServiceClient.prototype.iMUOrientation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/IMUOrientation',
      request,
      metadata || {},
      methodDescriptor_IMUService_IMUOrientation,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.IMUOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.IMUOrientationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.IMUServicePromiseClient.prototype.iMUOrientation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.IMUService/IMUOrientation',
      request,
      metadata || {},
      methodDescriptor_IMUService_IMUOrientation);
};


module.exports = proto.proto.api.component.v1;

