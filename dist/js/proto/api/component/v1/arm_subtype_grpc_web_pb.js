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
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.component = {};
proto.proto.api.component.v1 = require('./arm_subtype_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.ArmSubtypeServiceClient =
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
proto.proto.api.component.v1.ArmSubtypeServicePromiseClient =
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
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse>}
 */
const methodDescriptor_ArmSubtypeService_CurrentPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmSubtypeService/CurrentPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionRequest,
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse>}
 */
const methodInfo_ArmSubtypeService_CurrentPosition = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmSubtypeServiceClient.prototype.currentPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/CurrentPosition',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_CurrentPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmSubtypeServiceCurrentPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmSubtypeServicePromiseClient.prototype.currentPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/CurrentPosition',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_CurrentPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse>}
 */
const methodDescriptor_ArmSubtypeService_MoveToPosition = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmSubtypeService/MoveToPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionRequest,
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse>}
 */
const methodInfo_ArmSubtypeService_MoveToPosition = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmSubtypeServiceClient.prototype.moveToPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/MoveToPosition',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_MoveToPosition,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmSubtypeServiceMoveToPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmSubtypeServicePromiseClient.prototype.moveToPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/MoveToPosition',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_MoveToPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse>}
 */
const methodDescriptor_ArmSubtypeService_CurrentJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmSubtypeService/CurrentJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsRequest,
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse>}
 */
const methodInfo_ArmSubtypeService_CurrentJointPositions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmSubtypeServiceClient.prototype.currentJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/CurrentJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_CurrentJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmSubtypeServiceCurrentJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmSubtypeServicePromiseClient.prototype.currentJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/CurrentJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_CurrentJointPositions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse>}
 */
const methodDescriptor_ArmSubtypeService_MoveToJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmSubtypeService/MoveToJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsRequest,
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse>}
 */
const methodInfo_ArmSubtypeService_MoveToJointPositions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmSubtypeServiceClient.prototype.moveToJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/MoveToJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_MoveToJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmSubtypeServiceMoveToJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmSubtypeServicePromiseClient.prototype.moveToJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/MoveToJointPositions',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_MoveToJointPositions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse>}
 */
const methodDescriptor_ArmSubtypeService_JointMoveDelta = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ArmSubtypeService/JointMoveDelta',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaRequest,
  proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaRequest,
 *   !proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse>}
 */
const methodInfo_ArmSubtypeService_JointMoveDelta = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse,
  /**
   * @param {!proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ArmSubtypeServiceClient.prototype.jointMoveDelta =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/JointMoveDelta',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_JointMoveDelta,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ArmSubtypeServiceJointMoveDeltaResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ArmSubtypeServicePromiseClient.prototype.jointMoveDelta =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ArmSubtypeService/JointMoveDelta',
      request,
      metadata || {},
      methodDescriptor_ArmSubtypeService_JointMoveDelta);
};


module.exports = proto.proto.api.component.v1;

