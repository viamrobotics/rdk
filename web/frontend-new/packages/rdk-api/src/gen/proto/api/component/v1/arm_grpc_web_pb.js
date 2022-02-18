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
proto.proto.api.component.v1 = require('./arm_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.ArmServiceClient =
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
proto.proto.api.component.v1.ArmServicePromiseClient =
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
 *   !proto.proto.api.component.v1.ArmServiceGetEndPositionRequest,
 *   !proto.proto.api.component.v1.ArmServiceGetEndPositionResponse>}
 */
const methodDescriptor_ArmService_GetEndPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmService/GetEndPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmServiceGetEndPositionRequest,
  proto.proto.api.component.v1.ArmServiceGetEndPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmServiceGetEndPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmServiceGetEndPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmServiceGetEndPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ArmServiceGetEndPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmServiceGetEndPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmServiceClient.prototype.getEndPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/GetEndPosition',
      request,
      metadata || {},
      methodDescriptor_ArmService_GetEndPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmServiceGetEndPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmServiceGetEndPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmServicePromiseClient.prototype.getEndPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/GetEndPosition',
      request,
      metadata || {},
      methodDescriptor_ArmService_GetEndPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmServiceMoveToPositionRequest,
 *   !proto.proto.api.component.v1.ArmServiceMoveToPositionResponse>}
 */
const methodDescriptor_ArmService_MoveToPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmService/MoveToPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmServiceMoveToPositionRequest,
  proto.proto.api.component.v1.ArmServiceMoveToPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmServiceMoveToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmServiceMoveToPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmServiceMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ArmServiceMoveToPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmServiceMoveToPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmServiceClient.prototype.moveToPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/MoveToPosition',
      request,
      metadata || {},
      methodDescriptor_ArmService_MoveToPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmServiceMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmServiceMoveToPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmServicePromiseClient.prototype.moveToPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/MoveToPosition',
      request,
      metadata || {},
      methodDescriptor_ArmService_MoveToPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmServiceGetJointPositionsRequest,
 *   !proto.proto.api.component.v1.ArmServiceGetJointPositionsResponse>}
 */
const methodDescriptor_ArmService_GetJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmService/GetJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmServiceGetJointPositionsRequest,
  proto.proto.api.component.v1.ArmServiceGetJointPositionsResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmServiceGetJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmServiceGetJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmServiceGetJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ArmServiceGetJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmServiceGetJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmServiceClient.prototype.getJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/GetJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmService_GetJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmServiceGetJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmServiceGetJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmServicePromiseClient.prototype.getJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/GetJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmService_GetJointPositions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmServiceMoveToJointPositionsRequest,
 *   !proto.proto.api.component.v1.ArmServiceMoveToJointPositionsResponse>}
 */
const methodDescriptor_ArmService_MoveToJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmService/MoveToJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmServiceMoveToJointPositionsRequest,
  proto.proto.api.component.v1.ArmServiceMoveToJointPositionsResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmServiceMoveToJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmServiceMoveToJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmServiceMoveToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ArmServiceMoveToJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmServiceMoveToJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmServiceClient.prototype.moveToJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/MoveToJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmService_MoveToJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmServiceMoveToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmServiceMoveToJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmServicePromiseClient.prototype.moveToJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/MoveToJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmService_MoveToJointPositions);
};


module.exports = proto.proto.api.component.v1;

