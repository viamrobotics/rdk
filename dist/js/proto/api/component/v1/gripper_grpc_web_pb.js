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
proto.proto.api.component.v1 = require('./gripper_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.GripperServiceClient =
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
proto.proto.api.component.v1.GripperServicePromiseClient =
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
 *   !proto.proto.api.component.v1.GripperServiceOpenRequest,
 *   !proto.proto.api.component.v1.GripperServiceOpenResponse>}
 */
const methodDescriptor_GripperService_Open = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GripperService/Open',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GripperServiceOpenRequest,
  proto.proto.api.component.v1.GripperServiceOpenResponse,
  /**
   * @param {!proto.proto.api.component.v1.GripperServiceOpenRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GripperServiceOpenResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GripperServiceOpenRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GripperServiceOpenResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GripperServiceOpenResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GripperServiceClient.prototype.open =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GripperService/Open',
      request,
      metadata || {},
      methodDescriptor_GripperService_Open,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GripperServiceOpenRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GripperServiceOpenResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GripperServicePromiseClient.prototype.open =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GripperService/Open',
      request,
      metadata || {},
      methodDescriptor_GripperService_Open);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.GripperServiceGrabRequest,
 *   !proto.proto.api.component.v1.GripperServiceGrabResponse>}
 */
const methodDescriptor_GripperService_Grab = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.GripperService/Grab',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.GripperServiceGrabRequest,
  proto.proto.api.component.v1.GripperServiceGrabResponse,
  /**
   * @param {!proto.proto.api.component.v1.GripperServiceGrabRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.GripperServiceGrabResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.GripperServiceGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.GripperServiceGrabResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.GripperServiceGrabResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.GripperServiceClient.prototype.grab =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.GripperService/Grab',
      request,
      metadata || {},
      methodDescriptor_GripperService_Grab,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.GripperServiceGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.GripperServiceGrabResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.GripperServicePromiseClient.prototype.grab =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.GripperService/Grab',
      request,
      metadata || {},
      methodDescriptor_GripperService_Grab);
};


module.exports = proto.proto.api.component.v1;

