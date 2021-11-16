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

var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var google_api_httpbody_pb = require('../../../../google/api/httpbody_pb.js')

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
 *   !proto.proto.api.component.v1.ArmServiceCurrentPositionRequest,
 *   !proto.proto.api.component.v1.ArmServiceCurrentPositionResponse>}
 */
const methodDescriptor_ArmService_CurrentPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmService/CurrentPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmServiceCurrentPositionRequest,
  proto.proto.api.component.v1.ArmServiceCurrentPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmServiceCurrentPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmServiceCurrentPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmServiceCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ArmServiceCurrentPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmServiceCurrentPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmServiceClient.prototype.currentPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/CurrentPosition',
      request,
      metadata || {},
      methodDescriptor_ArmService_CurrentPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmServiceCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmServiceCurrentPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmServicePromiseClient.prototype.currentPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/CurrentPosition',
      request,
      metadata || {},
      methodDescriptor_ArmService_CurrentPosition);
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
 *   !proto.proto.api.component.v1.ArmServiceCurrentJointPositionsRequest,
 *   !proto.proto.api.component.v1.ArmServiceCurrentJointPositionsResponse>}
 */
const methodDescriptor_ArmService_CurrentJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmService/CurrentJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmServiceCurrentJointPositionsRequest,
  proto.proto.api.component.v1.ArmServiceCurrentJointPositionsResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmServiceCurrentJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmServiceCurrentJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmServiceCurrentJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ArmServiceCurrentJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmServiceCurrentJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmServiceClient.prototype.currentJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/CurrentJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmService_CurrentJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmServiceCurrentJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmServiceCurrentJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmServicePromiseClient.prototype.currentJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/CurrentJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmService_CurrentJointPositions);
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


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmServiceJointMoveDeltaRequest,
 *   !proto.proto.api.component.v1.ArmServiceJointMoveDeltaResponse>}
 */
const methodDescriptor_ArmService_JointMoveDelta = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmService/JointMoveDelta',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmServiceJointMoveDeltaRequest,
  proto.proto.api.component.v1.ArmServiceJointMoveDeltaResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmServiceJointMoveDeltaRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmServiceJointMoveDeltaResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmServiceJointMoveDeltaRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ArmServiceJointMoveDeltaResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmServiceJointMoveDeltaResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmServiceClient.prototype.jointMoveDelta =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/JointMoveDelta',
      request,
      metadata || {},
      methodDescriptor_ArmService_JointMoveDelta,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmServiceJointMoveDeltaRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmServiceJointMoveDeltaResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmServicePromiseClient.prototype.jointMoveDelta =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmService/JointMoveDelta',
      request,
      metadata || {},
      methodDescriptor_ArmService_JointMoveDelta);
};


module.exports = proto.proto.api.component.v1;

