/**
 * @fileoverview gRPC-Web generated client stub for proto.api.v1
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

var google_api_annotations_pb = require('../../../google/api/annotations_pb.js')

var google_api_httpbody_pb = require('../../../google/api/httpbody_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.v1 = require('./robot_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.v1.RobotServiceClient =
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
proto.proto.api.v1.RobotServicePromiseClient =
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
 *   !proto.proto.api.v1.StatusRequest,
 *   !proto.proto.api.v1.StatusResponse>}
 */
const methodDescriptor_RobotService_Status = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/Status',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.StatusRequest,
  proto.proto.api.v1.StatusResponse,
  /**
   * @param {!proto.proto.api.v1.StatusRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.StatusResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.StatusRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.StatusResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.StatusResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.status =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/Status',
      request,
      metadata || {},
      methodDescriptor_RobotService_Status,
      callback);
};


/**
 * @param {!proto.proto.api.v1.StatusRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.StatusResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.status =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/Status',
      request,
      metadata || {},
      methodDescriptor_RobotService_Status);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.StatusStreamRequest,
 *   !proto.proto.api.v1.StatusStreamResponse>}
 */
const methodDescriptor_RobotService_StatusStream = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/StatusStream',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.proto.api.v1.StatusStreamRequest,
  proto.proto.api.v1.StatusStreamResponse,
  /**
   * @param {!proto.proto.api.v1.StatusStreamRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.StatusStreamResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.StatusStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.StatusStreamResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.statusStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/StatusStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_StatusStream);
};


/**
 * @param {!proto.proto.api.v1.StatusStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.StatusStreamResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.statusStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/StatusStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_StatusStream);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ConfigRequest,
 *   !proto.proto.api.v1.ConfigResponse>}
 */
const methodDescriptor_RobotService_Config = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/Config',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ConfigRequest,
  proto.proto.api.v1.ConfigResponse,
  /**
   * @param {!proto.proto.api.v1.ConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.config =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/Config',
      request,
      metadata || {},
      methodDescriptor_RobotService_Config,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.config =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/Config',
      request,
      metadata || {},
      methodDescriptor_RobotService_Config);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.DoActionRequest,
 *   !proto.proto.api.v1.DoActionResponse>}
 */
const methodDescriptor_RobotService_DoAction = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/DoAction',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.DoActionRequest,
  proto.proto.api.v1.DoActionResponse,
  /**
   * @param {!proto.proto.api.v1.DoActionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.DoActionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.DoActionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.DoActionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.DoActionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.doAction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/DoAction',
      request,
      metadata || {},
      methodDescriptor_RobotService_DoAction,
      callback);
};


/**
 * @param {!proto.proto.api.v1.DoActionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.DoActionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.doAction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/DoAction',
      request,
      metadata || {},
      methodDescriptor_RobotService_DoAction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GantryCurrentPositionRequest,
 *   !proto.proto.api.v1.GantryCurrentPositionResponse>}
 */
const methodDescriptor_RobotService_GantryCurrentPosition = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GantryCurrentPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GantryCurrentPositionRequest,
  proto.proto.api.v1.GantryCurrentPositionResponse,
  /**
   * @param {!proto.proto.api.v1.GantryCurrentPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GantryCurrentPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GantryCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GantryCurrentPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GantryCurrentPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gantryCurrentPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GantryCurrentPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_GantryCurrentPosition,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GantryCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GantryCurrentPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gantryCurrentPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GantryCurrentPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_GantryCurrentPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GantryMoveToPositionRequest,
 *   !proto.proto.api.v1.GantryMoveToPositionResponse>}
 */
const methodDescriptor_RobotService_GantryMoveToPosition = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GantryMoveToPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GantryMoveToPositionRequest,
  proto.proto.api.v1.GantryMoveToPositionResponse,
  /**
   * @param {!proto.proto.api.v1.GantryMoveToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GantryMoveToPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GantryMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GantryMoveToPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GantryMoveToPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gantryMoveToPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GantryMoveToPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_GantryMoveToPosition,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GantryMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GantryMoveToPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gantryMoveToPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GantryMoveToPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_GantryMoveToPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ArmCurrentPositionRequest,
 *   !proto.proto.api.v1.ArmCurrentPositionResponse>}
 */
const methodDescriptor_RobotService_ArmCurrentPosition = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ArmCurrentPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ArmCurrentPositionRequest,
  proto.proto.api.v1.ArmCurrentPositionResponse,
  /**
   * @param {!proto.proto.api.v1.ArmCurrentPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ArmCurrentPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ArmCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ArmCurrentPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ArmCurrentPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.armCurrentPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmCurrentPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmCurrentPosition,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ArmCurrentPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ArmCurrentPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.armCurrentPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmCurrentPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmCurrentPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ArmMoveToPositionRequest,
 *   !proto.proto.api.v1.ArmMoveToPositionResponse>}
 */
const methodDescriptor_RobotService_ArmMoveToPosition = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ArmMoveToPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ArmMoveToPositionRequest,
  proto.proto.api.v1.ArmMoveToPositionResponse,
  /**
   * @param {!proto.proto.api.v1.ArmMoveToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ArmMoveToPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ArmMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ArmMoveToPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ArmMoveToPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.armMoveToPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmMoveToPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmMoveToPosition,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ArmMoveToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ArmMoveToPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.armMoveToPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmMoveToPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmMoveToPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ArmCurrentJointPositionsRequest,
 *   !proto.proto.api.v1.ArmCurrentJointPositionsResponse>}
 */
const methodDescriptor_RobotService_ArmCurrentJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ArmCurrentJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ArmCurrentJointPositionsRequest,
  proto.proto.api.v1.ArmCurrentJointPositionsResponse,
  /**
   * @param {!proto.proto.api.v1.ArmCurrentJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ArmCurrentJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ArmCurrentJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ArmCurrentJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ArmCurrentJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.armCurrentJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmCurrentJointPositions',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmCurrentJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ArmCurrentJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ArmCurrentJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.armCurrentJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmCurrentJointPositions',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmCurrentJointPositions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ArmMoveToJointPositionsRequest,
 *   !proto.proto.api.v1.ArmMoveToJointPositionsResponse>}
 */
const methodDescriptor_RobotService_ArmMoveToJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ArmMoveToJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ArmMoveToJointPositionsRequest,
  proto.proto.api.v1.ArmMoveToJointPositionsResponse,
  /**
   * @param {!proto.proto.api.v1.ArmMoveToJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ArmMoveToJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ArmMoveToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ArmMoveToJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ArmMoveToJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.armMoveToJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmMoveToJointPositions',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmMoveToJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ArmMoveToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ArmMoveToJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.armMoveToJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmMoveToJointPositions',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmMoveToJointPositions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ArmJointMoveDeltaRequest,
 *   !proto.proto.api.v1.ArmJointMoveDeltaResponse>}
 */
const methodDescriptor_RobotService_ArmJointMoveDelta = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ArmJointMoveDelta',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ArmJointMoveDeltaRequest,
  proto.proto.api.v1.ArmJointMoveDeltaResponse,
  /**
   * @param {!proto.proto.api.v1.ArmJointMoveDeltaRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ArmJointMoveDeltaResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ArmJointMoveDeltaRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ArmJointMoveDeltaResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ArmJointMoveDeltaResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.armJointMoveDelta =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmJointMoveDelta',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmJointMoveDelta,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ArmJointMoveDeltaRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ArmJointMoveDeltaResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.armJointMoveDelta =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ArmJointMoveDelta',
      request,
      metadata || {},
      methodDescriptor_RobotService_ArmJointMoveDelta);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseMoveStraightRequest,
 *   !proto.proto.api.v1.BaseMoveStraightResponse>}
 */
const methodDescriptor_RobotService_BaseMoveStraight = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseMoveStraight',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseMoveStraightRequest,
  proto.proto.api.v1.BaseMoveStraightResponse,
  /**
   * @param {!proto.proto.api.v1.BaseMoveStraightRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseMoveStraightResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseMoveStraightRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseMoveStraightResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseMoveStraightResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseMoveStraight =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseMoveStraight',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseMoveStraight,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseMoveStraightRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseMoveStraightResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseMoveStraight =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseMoveStraight',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseMoveStraight);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseSpinRequest,
 *   !proto.proto.api.v1.BaseSpinResponse>}
 */
const methodDescriptor_RobotService_BaseSpin = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseSpin',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseSpinRequest,
  proto.proto.api.v1.BaseSpinResponse,
  /**
   * @param {!proto.proto.api.v1.BaseSpinRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseSpinResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseSpinRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseSpinResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseSpinResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseSpin =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseSpin',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseSpin,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseSpinRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseSpinResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseSpin =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseSpin',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseSpin);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseStopRequest,
 *   !proto.proto.api.v1.BaseStopResponse>}
 */
const methodDescriptor_RobotService_BaseStop = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseStop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseStopRequest,
  proto.proto.api.v1.BaseStopResponse,
  /**
   * @param {!proto.proto.api.v1.BaseStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseStop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseStop,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseStopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseStop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseStop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BaseWidthMillisRequest,
 *   !proto.proto.api.v1.BaseWidthMillisResponse>}
 */
const methodDescriptor_RobotService_BaseWidthMillis = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BaseWidthMillis',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BaseWidthMillisRequest,
  proto.proto.api.v1.BaseWidthMillisResponse,
  /**
   * @param {!proto.proto.api.v1.BaseWidthMillisRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BaseWidthMillisResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BaseWidthMillisRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BaseWidthMillisResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BaseWidthMillisResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.baseWidthMillis =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseWidthMillis',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseWidthMillis,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BaseWidthMillisRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BaseWidthMillisResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.baseWidthMillis =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BaseWidthMillis',
      request,
      metadata || {},
      methodDescriptor_RobotService_BaseWidthMillis);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GripperOpenRequest,
 *   !proto.proto.api.v1.GripperOpenResponse>}
 */
const methodDescriptor_RobotService_GripperOpen = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GripperOpen',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GripperOpenRequest,
  proto.proto.api.v1.GripperOpenResponse,
  /**
   * @param {!proto.proto.api.v1.GripperOpenRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GripperOpenResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GripperOpenRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GripperOpenResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GripperOpenResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gripperOpen =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GripperOpen',
      request,
      metadata || {},
      methodDescriptor_RobotService_GripperOpen,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GripperOpenRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GripperOpenResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gripperOpen =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GripperOpen',
      request,
      metadata || {},
      methodDescriptor_RobotService_GripperOpen);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GripperGrabRequest,
 *   !proto.proto.api.v1.GripperGrabResponse>}
 */
const methodDescriptor_RobotService_GripperGrab = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GripperGrab',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GripperGrabRequest,
  proto.proto.api.v1.GripperGrabResponse,
  /**
   * @param {!proto.proto.api.v1.GripperGrabRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GripperGrabResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GripperGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GripperGrabResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GripperGrabResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gripperGrab =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GripperGrab',
      request,
      metadata || {},
      methodDescriptor_RobotService_GripperGrab,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GripperGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GripperGrabResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gripperGrab =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GripperGrab',
      request,
      metadata || {},
      methodDescriptor_RobotService_GripperGrab);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CameraFrameRequest,
 *   !proto.proto.api.v1.CameraFrameResponse>}
 */
const methodDescriptor_RobotService_CameraFrame = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CameraFrame',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CameraFrameRequest,
  proto.proto.api.v1.CameraFrameResponse,
  /**
   * @param {!proto.proto.api.v1.CameraFrameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CameraFrameResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CameraFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CameraFrameResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CameraFrameResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.cameraFrame =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CameraFrame',
      request,
      metadata || {},
      methodDescriptor_RobotService_CameraFrame,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CameraFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CameraFrameResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.cameraFrame =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CameraFrame',
      request,
      metadata || {},
      methodDescriptor_RobotService_CameraFrame);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CameraRenderFrameRequest,
 *   !proto.google.api.HttpBody>}
 */
const methodDescriptor_RobotService_CameraRenderFrame = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CameraRenderFrame',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CameraRenderFrameRequest,
  google_api_httpbody_pb.HttpBody,
  /**
   * @param {!proto.proto.api.v1.CameraRenderFrameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_httpbody_pb.HttpBody.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CameraRenderFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.HttpBody)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.HttpBody>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.cameraRenderFrame =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CameraRenderFrame',
      request,
      metadata || {},
      methodDescriptor_RobotService_CameraRenderFrame,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CameraRenderFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.HttpBody>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.cameraRenderFrame =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CameraRenderFrame',
      request,
      metadata || {},
      methodDescriptor_RobotService_CameraRenderFrame);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.PointCloudRequest,
 *   !proto.proto.api.v1.PointCloudResponse>}
 */
const methodDescriptor_RobotService_PointCloud = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/PointCloud',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.PointCloudRequest,
  proto.proto.api.v1.PointCloudResponse,
  /**
   * @param {!proto.proto.api.v1.PointCloudRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.PointCloudResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.PointCloudRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.PointCloudResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.PointCloudResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.pointCloud =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/PointCloud',
      request,
      metadata || {},
      methodDescriptor_RobotService_PointCloud,
      callback);
};


/**
 * @param {!proto.proto.api.v1.PointCloudRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.PointCloudResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.pointCloud =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/PointCloud',
      request,
      metadata || {},
      methodDescriptor_RobotService_PointCloud);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ObjectPointCloudsRequest,
 *   !proto.proto.api.v1.ObjectPointCloudsResponse>}
 */
const methodDescriptor_RobotService_ObjectPointClouds = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ObjectPointClouds',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ObjectPointCloudsRequest,
  proto.proto.api.v1.ObjectPointCloudsResponse,
  /**
   * @param {!proto.proto.api.v1.ObjectPointCloudsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ObjectPointCloudsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ObjectPointCloudsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ObjectPointCloudsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ObjectPointCloudsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.objectPointClouds =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ObjectPointClouds',
      request,
      metadata || {},
      methodDescriptor_RobotService_ObjectPointClouds,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ObjectPointCloudsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ObjectPointCloudsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.objectPointClouds =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ObjectPointClouds',
      request,
      metadata || {},
      methodDescriptor_RobotService_ObjectPointClouds);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.LidarInfoRequest,
 *   !proto.proto.api.v1.LidarInfoResponse>}
 */
const methodDescriptor_RobotService_LidarInfo = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/LidarInfo',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.LidarInfoRequest,
  proto.proto.api.v1.LidarInfoResponse,
  /**
   * @param {!proto.proto.api.v1.LidarInfoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.LidarInfoResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.LidarInfoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.LidarInfoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.LidarInfoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.lidarInfo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarInfo',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarInfo,
      callback);
};


/**
 * @param {!proto.proto.api.v1.LidarInfoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.LidarInfoResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.lidarInfo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarInfo',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarInfo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.LidarStartRequest,
 *   !proto.proto.api.v1.LidarStartResponse>}
 */
const methodDescriptor_RobotService_LidarStart = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/LidarStart',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.LidarStartRequest,
  proto.proto.api.v1.LidarStartResponse,
  /**
   * @param {!proto.proto.api.v1.LidarStartRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.LidarStartResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.LidarStartRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.LidarStartResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.LidarStartResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.lidarStart =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarStart',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarStart,
      callback);
};


/**
 * @param {!proto.proto.api.v1.LidarStartRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.LidarStartResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.lidarStart =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarStart',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarStart);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.LidarStopRequest,
 *   !proto.proto.api.v1.LidarStopResponse>}
 */
const methodDescriptor_RobotService_LidarStop = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/LidarStop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.LidarStopRequest,
  proto.proto.api.v1.LidarStopResponse,
  /**
   * @param {!proto.proto.api.v1.LidarStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.LidarStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.LidarStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.LidarStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.LidarStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.lidarStop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarStop,
      callback);
};


/**
 * @param {!proto.proto.api.v1.LidarStopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.LidarStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.lidarStop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarStop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.LidarScanRequest,
 *   !proto.proto.api.v1.LidarScanResponse>}
 */
const methodDescriptor_RobotService_LidarScan = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/LidarScan',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.LidarScanRequest,
  proto.proto.api.v1.LidarScanResponse,
  /**
   * @param {!proto.proto.api.v1.LidarScanRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.LidarScanResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.LidarScanRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.LidarScanResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.LidarScanResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.lidarScan =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarScan',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarScan,
      callback);
};


/**
 * @param {!proto.proto.api.v1.LidarScanRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.LidarScanResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.lidarScan =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarScan',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarScan);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.LidarRangeRequest,
 *   !proto.proto.api.v1.LidarRangeResponse>}
 */
const methodDescriptor_RobotService_LidarRange = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/LidarRange',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.LidarRangeRequest,
  proto.proto.api.v1.LidarRangeResponse,
  /**
   * @param {!proto.proto.api.v1.LidarRangeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.LidarRangeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.LidarRangeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.LidarRangeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.LidarRangeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.lidarRange =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarRange',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarRange,
      callback);
};


/**
 * @param {!proto.proto.api.v1.LidarRangeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.LidarRangeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.lidarRange =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarRange',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarRange);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.LidarBoundsRequest,
 *   !proto.proto.api.v1.LidarBoundsResponse>}
 */
const methodDescriptor_RobotService_LidarBounds = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/LidarBounds',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.LidarBoundsRequest,
  proto.proto.api.v1.LidarBoundsResponse,
  /**
   * @param {!proto.proto.api.v1.LidarBoundsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.LidarBoundsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.LidarBoundsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.LidarBoundsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.LidarBoundsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.lidarBounds =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarBounds',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarBounds,
      callback);
};


/**
 * @param {!proto.proto.api.v1.LidarBoundsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.LidarBoundsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.lidarBounds =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarBounds',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarBounds);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.LidarAngularResolutionRequest,
 *   !proto.proto.api.v1.LidarAngularResolutionResponse>}
 */
const methodDescriptor_RobotService_LidarAngularResolution = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/LidarAngularResolution',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.LidarAngularResolutionRequest,
  proto.proto.api.v1.LidarAngularResolutionResponse,
  /**
   * @param {!proto.proto.api.v1.LidarAngularResolutionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.LidarAngularResolutionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.LidarAngularResolutionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.LidarAngularResolutionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.LidarAngularResolutionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.lidarAngularResolution =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarAngularResolution',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarAngularResolution,
      callback);
};


/**
 * @param {!proto.proto.api.v1.LidarAngularResolutionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.LidarAngularResolutionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.lidarAngularResolution =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/LidarAngularResolution',
      request,
      metadata || {},
      methodDescriptor_RobotService_LidarAngularResolution);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardStatusRequest,
 *   !proto.proto.api.v1.BoardStatusResponse>}
 */
const methodDescriptor_RobotService_BoardStatus = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardStatus',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardStatusRequest,
  proto.proto.api.v1.BoardStatusResponse,
  /**
   * @param {!proto.proto.api.v1.BoardStatusRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardStatusResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardStatusRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardStatusResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardStatusResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardStatus =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardStatus',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardStatus,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardStatusRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardStatusResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardStatus =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardStatus',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardStatus);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardGPIOSetRequest,
 *   !proto.proto.api.v1.BoardGPIOSetResponse>}
 */
const methodDescriptor_RobotService_BoardGPIOSet = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardGPIOSet',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardGPIOSetRequest,
  proto.proto.api.v1.BoardGPIOSetResponse,
  /**
   * @param {!proto.proto.api.v1.BoardGPIOSetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardGPIOSetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardGPIOSetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardGPIOSetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardGPIOSetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardGPIOSet =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOSet,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardGPIOSetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardGPIOSetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardGPIOSet =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOSet);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardGPIOGetRequest,
 *   !proto.proto.api.v1.BoardGPIOGetResponse>}
 */
const methodDescriptor_RobotService_BoardGPIOGet = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardGPIOGet',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardGPIOGetRequest,
  proto.proto.api.v1.BoardGPIOGetResponse,
  /**
   * @param {!proto.proto.api.v1.BoardGPIOGetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardGPIOGetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardGPIOGetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardGPIOGetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardGPIOGetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardGPIOGet =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOGet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOGet,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardGPIOGetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardGPIOGetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardGPIOGet =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardGPIOGet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardGPIOGet);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardPWMSetRequest,
 *   !proto.proto.api.v1.BoardPWMSetResponse>}
 */
const methodDescriptor_RobotService_BoardPWMSet = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardPWMSet',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardPWMSetRequest,
  proto.proto.api.v1.BoardPWMSetResponse,
  /**
   * @param {!proto.proto.api.v1.BoardPWMSetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardPWMSetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardPWMSetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardPWMSetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardPWMSetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardPWMSet =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSet,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardPWMSetRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardPWMSetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardPWMSet =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSet',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSet);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardPWMSetFrequencyRequest,
 *   !proto.proto.api.v1.BoardPWMSetFrequencyResponse>}
 */
const methodDescriptor_RobotService_BoardPWMSetFrequency = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardPWMSetFrequency',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardPWMSetFrequencyRequest,
  proto.proto.api.v1.BoardPWMSetFrequencyResponse,
  /**
   * @param {!proto.proto.api.v1.BoardPWMSetFrequencyRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardPWMSetFrequencyResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardPWMSetFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardPWMSetFrequencyResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardPWMSetFrequencyResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardPWMSetFrequency =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSetFrequency',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSetFrequency,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardPWMSetFrequencyRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardPWMSetFrequencyResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardPWMSetFrequency =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardPWMSetFrequency',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardPWMSetFrequency);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardAnalogReaderReadRequest,
 *   !proto.proto.api.v1.BoardAnalogReaderReadResponse>}
 */
const methodDescriptor_RobotService_BoardAnalogReaderRead = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardAnalogReaderRead',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardAnalogReaderReadRequest,
  proto.proto.api.v1.BoardAnalogReaderReadResponse,
  /**
   * @param {!proto.proto.api.v1.BoardAnalogReaderReadRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardAnalogReaderReadResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardAnalogReaderReadRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardAnalogReaderReadResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardAnalogReaderReadResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardAnalogReaderRead =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardAnalogReaderRead',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardAnalogReaderRead,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardAnalogReaderReadRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardAnalogReaderReadResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardAnalogReaderRead =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardAnalogReaderRead',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardAnalogReaderRead);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardDigitalInterruptConfigRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptConfigResponse>}
 */
const methodDescriptor_RobotService_BoardDigitalInterruptConfig = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardDigitalInterruptConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardDigitalInterruptConfigRequest,
  proto.proto.api.v1.BoardDigitalInterruptConfigResponse,
  /**
   * @param {!proto.proto.api.v1.BoardDigitalInterruptConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardDigitalInterruptConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardDigitalInterruptConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardDigitalInterruptConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardDigitalInterruptConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptConfig,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardDigitalInterruptConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardDigitalInterruptConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardDigitalInterruptValueRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptValueResponse>}
 */
const methodDescriptor_RobotService_BoardDigitalInterruptValue = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardDigitalInterruptValue',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardDigitalInterruptValueRequest,
  proto.proto.api.v1.BoardDigitalInterruptValueResponse,
  /**
   * @param {!proto.proto.api.v1.BoardDigitalInterruptValueRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardDigitalInterruptValueResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardDigitalInterruptValueResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardDigitalInterruptValueResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardDigitalInterruptValue =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptValue,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptValueRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardDigitalInterruptValueResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardDigitalInterruptValue =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptValue',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptValue);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardDigitalInterruptTickRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptTickResponse>}
 */
const methodDescriptor_RobotService_BoardDigitalInterruptTick = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardDigitalInterruptTick',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardDigitalInterruptTickRequest,
  proto.proto.api.v1.BoardDigitalInterruptTickResponse,
  /**
   * @param {!proto.proto.api.v1.BoardDigitalInterruptTickRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardDigitalInterruptTickResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptTickRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.BoardDigitalInterruptTickResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardDigitalInterruptTickResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardDigitalInterruptTick =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptTick',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptTick,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardDigitalInterruptTickRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardDigitalInterruptTickResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardDigitalInterruptTick =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardDigitalInterruptTick',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardDigitalInterruptTick);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.SensorReadingsRequest,
 *   !proto.proto.api.v1.SensorReadingsResponse>}
 */
const methodDescriptor_RobotService_SensorReadings = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/SensorReadings',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.SensorReadingsRequest,
  proto.proto.api.v1.SensorReadingsResponse,
  /**
   * @param {!proto.proto.api.v1.SensorReadingsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.SensorReadingsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.SensorReadingsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.SensorReadingsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.SensorReadingsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.sensorReadings =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/SensorReadings',
      request,
      metadata || {},
      methodDescriptor_RobotService_SensorReadings,
      callback);
};


/**
 * @param {!proto.proto.api.v1.SensorReadingsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.SensorReadingsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.sensorReadings =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/SensorReadings',
      request,
      metadata || {},
      methodDescriptor_RobotService_SensorReadings);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassHeadingRequest,
 *   !proto.proto.api.v1.CompassHeadingResponse>}
 */
const methodDescriptor_RobotService_CompassHeading = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassHeading',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassHeadingRequest,
  proto.proto.api.v1.CompassHeadingResponse,
  /**
   * @param {!proto.proto.api.v1.CompassHeadingRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassHeadingResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassHeadingRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassHeadingResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassHeadingResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassHeading =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassHeading',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassHeading,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassHeadingRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassHeadingResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassHeading =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassHeading',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassHeading);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassStartCalibrationRequest,
 *   !proto.proto.api.v1.CompassStartCalibrationResponse>}
 */
const methodDescriptor_RobotService_CompassStartCalibration = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassStartCalibration',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassStartCalibrationRequest,
  proto.proto.api.v1.CompassStartCalibrationResponse,
  /**
   * @param {!proto.proto.api.v1.CompassStartCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassStartCalibrationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassStartCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassStartCalibrationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassStartCalibrationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassStartCalibration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStartCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStartCalibration,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassStartCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassStartCalibrationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassStartCalibration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStartCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStartCalibration);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassStopCalibrationRequest,
 *   !proto.proto.api.v1.CompassStopCalibrationResponse>}
 */
const methodDescriptor_RobotService_CompassStopCalibration = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassStopCalibration',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassStopCalibrationRequest,
  proto.proto.api.v1.CompassStopCalibrationResponse,
  /**
   * @param {!proto.proto.api.v1.CompassStopCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassStopCalibrationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassStopCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassStopCalibrationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassStopCalibrationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassStopCalibration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStopCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStopCalibration,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassStopCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassStopCalibrationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassStopCalibration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassStopCalibration',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassStopCalibration);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.CompassMarkRequest,
 *   !proto.proto.api.v1.CompassMarkResponse>}
 */
const methodDescriptor_RobotService_CompassMark = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/CompassMark',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CompassMarkRequest,
  proto.proto.api.v1.CompassMarkResponse,
  /**
   * @param {!proto.proto.api.v1.CompassMarkRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.CompassMarkResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CompassMarkRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.CompassMarkResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.CompassMarkResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.compassMark =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassMark',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassMark,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CompassMarkRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.CompassMarkResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.compassMark =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/CompassMark',
      request,
      metadata || {},
      methodDescriptor_RobotService_CompassMark);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ForceMatrixMatrixRequest,
 *   !proto.proto.api.v1.ForceMatrixMatrixResponse>}
 */
const methodDescriptor_RobotService_ForceMatrixMatrix = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ForceMatrixMatrix',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ForceMatrixMatrixRequest,
  proto.proto.api.v1.ForceMatrixMatrixResponse,
  /**
   * @param {!proto.proto.api.v1.ForceMatrixMatrixRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ForceMatrixMatrixResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ForceMatrixMatrixRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ForceMatrixMatrixResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ForceMatrixMatrixResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.forceMatrixMatrix =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixMatrix',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixMatrix,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ForceMatrixMatrixRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ForceMatrixMatrixResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.forceMatrixMatrix =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixMatrix',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixMatrix);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ForceMatrixSlipDetectionRequest,
 *   !proto.proto.api.v1.ForceMatrixSlipDetectionResponse>}
 */
const methodDescriptor_RobotService_ForceMatrixSlipDetection = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ForceMatrixSlipDetection',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ForceMatrixSlipDetectionRequest,
  proto.proto.api.v1.ForceMatrixSlipDetectionResponse,
  /**
   * @param {!proto.proto.api.v1.ForceMatrixSlipDetectionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ForceMatrixSlipDetectionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ForceMatrixSlipDetectionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ForceMatrixSlipDetectionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ForceMatrixSlipDetectionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.forceMatrixSlipDetection =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixSlipDetection',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixSlipDetection,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ForceMatrixSlipDetectionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ForceMatrixSlipDetectionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.forceMatrixSlipDetection =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ForceMatrixSlipDetection',
      request,
      metadata || {},
      methodDescriptor_RobotService_ForceMatrixSlipDetection);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ExecuteFunctionRequest,
 *   !proto.proto.api.v1.ExecuteFunctionResponse>}
 */
const methodDescriptor_RobotService_ExecuteFunction = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ExecuteFunction',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ExecuteFunctionRequest,
  proto.proto.api.v1.ExecuteFunctionResponse,
  /**
   * @param {!proto.proto.api.v1.ExecuteFunctionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ExecuteFunctionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ExecuteFunctionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ExecuteFunctionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ExecuteFunctionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.executeFunction =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteFunction',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteFunction,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ExecuteFunctionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ExecuteFunctionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.executeFunction =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteFunction',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteFunction);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ExecuteSourceRequest,
 *   !proto.proto.api.v1.ExecuteSourceResponse>}
 */
const methodDescriptor_RobotService_ExecuteSource = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ExecuteSource',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ExecuteSourceRequest,
  proto.proto.api.v1.ExecuteSourceResponse,
  /**
   * @param {!proto.proto.api.v1.ExecuteSourceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ExecuteSourceResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ExecuteSourceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ExecuteSourceResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ExecuteSourceResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.executeSource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteSource',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteSource,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ExecuteSourceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ExecuteSourceResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.executeSource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ExecuteSource',
      request,
      metadata || {},
      methodDescriptor_RobotService_ExecuteSource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ServoMoveRequest,
 *   !proto.proto.api.v1.ServoMoveResponse>}
 */
const methodDescriptor_RobotService_ServoMove = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ServoMove',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ServoMoveRequest,
  proto.proto.api.v1.ServoMoveResponse,
  /**
   * @param {!proto.proto.api.v1.ServoMoveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ServoMoveResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ServoMoveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ServoMoveResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ServoMoveResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.servoMove =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ServoMove',
      request,
      metadata || {},
      methodDescriptor_RobotService_ServoMove,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ServoMoveRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ServoMoveResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.servoMove =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ServoMove',
      request,
      metadata || {},
      methodDescriptor_RobotService_ServoMove);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ServoCurrentRequest,
 *   !proto.proto.api.v1.ServoCurrentResponse>}
 */
const methodDescriptor_RobotService_ServoCurrent = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ServoCurrent',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ServoCurrentRequest,
  proto.proto.api.v1.ServoCurrentResponse,
  /**
   * @param {!proto.proto.api.v1.ServoCurrentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ServoCurrentResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ServoCurrentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ServoCurrentResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ServoCurrentResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.servoCurrent =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ServoCurrent',
      request,
      metadata || {},
      methodDescriptor_RobotService_ServoCurrent,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ServoCurrentRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ServoCurrentResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.servoCurrent =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ServoCurrent',
      request,
      metadata || {},
      methodDescriptor_RobotService_ServoCurrent);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorGetPIDConfigRequest,
 *   !proto.proto.api.v1.MotorGetPIDConfigResponse>}
 */
const methodDescriptor_RobotService_MotorGetPIDConfig = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorGetPIDConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorGetPIDConfigRequest,
  proto.proto.api.v1.MotorGetPIDConfigResponse,
  /**
   * @param {!proto.proto.api.v1.MotorGetPIDConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorGetPIDConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorGetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorGetPIDConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorGetPIDConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorGetPIDConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGetPIDConfig,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorGetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorGetPIDConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorGetPIDConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGetPIDConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorSetPIDConfigRequest,
 *   !proto.proto.api.v1.MotorSetPIDConfigResponse>}
 */
const methodDescriptor_RobotService_MotorSetPIDConfig = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorSetPIDConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorSetPIDConfigRequest,
  proto.proto.api.v1.MotorSetPIDConfigResponse,
  /**
   * @param {!proto.proto.api.v1.MotorSetPIDConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorSetPIDConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorSetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorSetPIDConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorSetPIDConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorSetPIDConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorSetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorSetPIDConfig,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorSetPIDConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorSetPIDConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorSetPIDConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorSetPIDConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorSetPIDConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorPIDStepRequest,
 *   !proto.proto.api.v1.MotorPIDStepResponse>}
 */
const methodDescriptor_RobotService_MotorPIDStep = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorPIDStep',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.proto.api.v1.MotorPIDStepRequest,
  proto.proto.api.v1.MotorPIDStepResponse,
  /**
   * @param {!proto.proto.api.v1.MotorPIDStepRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorPIDStepResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorPIDStepRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorPIDStepResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorPIDStep =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPIDStep',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPIDStep);
};


/**
 * @param {!proto.proto.api.v1.MotorPIDStepRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorPIDStepResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorPIDStep =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPIDStep',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPIDStep);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorPowerRequest,
 *   !proto.proto.api.v1.MotorPowerResponse>}
 */
const methodDescriptor_RobotService_MotorPower = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorPower',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorPowerRequest,
  proto.proto.api.v1.MotorPowerResponse,
  /**
   * @param {!proto.proto.api.v1.MotorPowerRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorPowerResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorPowerRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorPowerResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorPowerResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorPower =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPower',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPower,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorPowerRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorPowerResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorPower =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPower',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPower);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorGoRequest,
 *   !proto.proto.api.v1.MotorGoResponse>}
 */
const methodDescriptor_RobotService_MotorGo = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorGo',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorGoRequest,
  proto.proto.api.v1.MotorGoResponse,
  /**
   * @param {!proto.proto.api.v1.MotorGoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorGoResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorGoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorGoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorGoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorGo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGo',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGo,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorGoRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorGoResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorGo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGo',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorGoForRequest,
 *   !proto.proto.api.v1.MotorGoForResponse>}
 */
const methodDescriptor_RobotService_MotorGoFor = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorGoFor',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorGoForRequest,
  proto.proto.api.v1.MotorGoForResponse,
  /**
   * @param {!proto.proto.api.v1.MotorGoForRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorGoForResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorGoForRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorGoForResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorGoForResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorGoFor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGoFor',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGoFor,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorGoForRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorGoForResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorGoFor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGoFor',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGoFor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorGoToRequest,
 *   !proto.proto.api.v1.MotorGoToResponse>}
 */
const methodDescriptor_RobotService_MotorGoTo = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorGoTo',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorGoToRequest,
  proto.proto.api.v1.MotorGoToResponse,
  /**
   * @param {!proto.proto.api.v1.MotorGoToRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorGoToResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorGoToRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorGoToResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorGoToResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorGoTo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGoTo',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGoTo,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorGoToRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorGoToResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorGoTo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGoTo',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGoTo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorGoTillStopRequest,
 *   !proto.proto.api.v1.MotorGoTillStopResponse>}
 */
const methodDescriptor_RobotService_MotorGoTillStop = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorGoTillStop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorGoTillStopRequest,
  proto.proto.api.v1.MotorGoTillStopResponse,
  /**
   * @param {!proto.proto.api.v1.MotorGoTillStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorGoTillStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorGoTillStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorGoTillStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorGoTillStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorGoTillStop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGoTillStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGoTillStop,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorGoTillStopRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorGoTillStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorGoTillStop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorGoTillStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorGoTillStop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorZeroRequest,
 *   !proto.proto.api.v1.MotorZeroResponse>}
 */
const methodDescriptor_RobotService_MotorZero = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorZero',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorZeroRequest,
  proto.proto.api.v1.MotorZeroResponse,
  /**
   * @param {!proto.proto.api.v1.MotorZeroRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorZeroResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorZeroRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorZeroResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorZeroResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorZero =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorZero',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorZero,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorZeroRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorZeroResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorZero =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorZero',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorZero);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorPositionRequest,
 *   !proto.proto.api.v1.MotorPositionResponse>}
 */
const methodDescriptor_RobotService_MotorPosition = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorPositionRequest,
  proto.proto.api.v1.MotorPositionResponse,
  /**
   * @param {!proto.proto.api.v1.MotorPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPosition,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorPositionSupportedRequest,
 *   !proto.proto.api.v1.MotorPositionSupportedResponse>}
 */
const methodDescriptor_RobotService_MotorPositionSupported = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorPositionSupported',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorPositionSupportedRequest,
  proto.proto.api.v1.MotorPositionSupportedResponse,
  /**
   * @param {!proto.proto.api.v1.MotorPositionSupportedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorPositionSupportedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorPositionSupportedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorPositionSupportedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorPositionSupportedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorPositionSupported =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPositionSupported',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPositionSupported,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorPositionSupportedRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorPositionSupportedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorPositionSupported =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorPositionSupported',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorPositionSupported);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorOffRequest,
 *   !proto.proto.api.v1.MotorOffResponse>}
 */
const methodDescriptor_RobotService_MotorOff = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorOff',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorOffRequest,
  proto.proto.api.v1.MotorOffResponse,
  /**
   * @param {!proto.proto.api.v1.MotorOffRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorOffResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorOffRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorOffResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorOffResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorOff =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorOff',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorOff,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorOffRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorOffResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorOff =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorOff',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorOff);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MotorIsOnRequest,
 *   !proto.proto.api.v1.MotorIsOnResponse>}
 */
const methodDescriptor_RobotService_MotorIsOn = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MotorIsOn',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MotorIsOnRequest,
  proto.proto.api.v1.MotorIsOnResponse,
  /**
   * @param {!proto.proto.api.v1.MotorIsOnRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MotorIsOnResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MotorIsOnRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.MotorIsOnResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MotorIsOnResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.motorIsOn =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorIsOn',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorIsOn,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MotorIsOnRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MotorIsOnResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.motorIsOn =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MotorIsOn',
      request,
      metadata || {},
      methodDescriptor_RobotService_MotorIsOn);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.InputControllerControlsRequest,
 *   !proto.proto.api.v1.InputControllerControlsResponse>}
 */
const methodDescriptor_RobotService_InputControllerControls = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/InputControllerControls',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.InputControllerControlsRequest,
  proto.proto.api.v1.InputControllerControlsResponse,
  /**
   * @param {!proto.proto.api.v1.InputControllerControlsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.InputControllerControlsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.InputControllerControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.InputControllerControlsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerControlsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.inputControllerControls =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerControls',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerControls,
      callback);
};


/**
 * @param {!proto.proto.api.v1.InputControllerControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.InputControllerControlsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.inputControllerControls =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerControls',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerControls);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.InputControllerLastEventsRequest,
 *   !proto.proto.api.v1.InputControllerLastEventsResponse>}
 */
const methodDescriptor_RobotService_InputControllerLastEvents = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/InputControllerLastEvents',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.InputControllerLastEventsRequest,
  proto.proto.api.v1.InputControllerLastEventsResponse,
  /**
   * @param {!proto.proto.api.v1.InputControllerLastEventsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.InputControllerLastEventsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.InputControllerLastEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.InputControllerLastEventsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerLastEventsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.inputControllerLastEvents =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerLastEvents',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerLastEvents,
      callback);
};


/**
 * @param {!proto.proto.api.v1.InputControllerLastEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.InputControllerLastEventsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.inputControllerLastEvents =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerLastEvents',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerLastEvents);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.InputControllerEventStreamRequest,
 *   !proto.proto.api.v1.InputControllerEvent>}
 */
const methodDescriptor_RobotService_InputControllerEventStream = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/InputControllerEventStream',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.proto.api.v1.InputControllerEventStreamRequest,
  proto.proto.api.v1.InputControllerEvent,
  /**
   * @param {!proto.proto.api.v1.InputControllerEventStreamRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.InputControllerEvent.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.InputControllerEventStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerEvent>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.inputControllerEventStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerEventStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerEventStream);
};


/**
 * @param {!proto.proto.api.v1.InputControllerEventStreamRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.InputControllerEvent>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.inputControllerEventStream =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.v1.RobotService/InputControllerEventStream',
      request,
      metadata || {},
      methodDescriptor_RobotService_InputControllerEventStream);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ResourceRunCommandRequest,
 *   !proto.proto.api.v1.ResourceRunCommandResponse>}
 */
const methodDescriptor_RobotService_ResourceRunCommand = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ResourceRunCommand',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ResourceRunCommandRequest,
  proto.proto.api.v1.ResourceRunCommandResponse,
  /**
   * @param {!proto.proto.api.v1.ResourceRunCommandRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ResourceRunCommandResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ResourceRunCommandRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ResourceRunCommandResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ResourceRunCommandResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.resourceRunCommand =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ResourceRunCommand',
      request,
      metadata || {},
      methodDescriptor_RobotService_ResourceRunCommand,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ResourceRunCommandRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ResourceRunCommandResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.resourceRunCommand =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ResourceRunCommand',
      request,
      metadata || {},
      methodDescriptor_RobotService_ResourceRunCommand);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.FrameServiceConfigRequest,
 *   !proto.proto.api.v1.FrameServiceConfigResponse>}
 */
const methodDescriptor_RobotService_FrameServiceConfig = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/FrameServiceConfig',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.FrameServiceConfigRequest,
  proto.proto.api.v1.FrameServiceConfigResponse,
  /**
   * @param {!proto.proto.api.v1.FrameServiceConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.FrameServiceConfigResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.FrameServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.FrameServiceConfigResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.FrameServiceConfigResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.frameServiceConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/FrameServiceConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_FrameServiceConfig,
      callback);
};


/**
 * @param {!proto.proto.api.v1.FrameServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.FrameServiceConfigResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.frameServiceConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/FrameServiceConfig',
      request,
      metadata || {},
      methodDescriptor_RobotService_FrameServiceConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceModeRequest,
 *   !proto.proto.api.v1.NavigationServiceModeResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceMode = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceMode',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceModeRequest,
  proto.proto.api.v1.NavigationServiceModeResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceModeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceModeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceModeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceMode =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceMode,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceModeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceModeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceMode =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceMode);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceSetModeRequest,
 *   !proto.proto.api.v1.NavigationServiceSetModeResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceSetMode = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceSetMode',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceSetModeRequest,
  proto.proto.api.v1.NavigationServiceSetModeResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceSetModeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceSetModeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceSetModeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceSetMode =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceSetMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceSetMode,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceSetModeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceSetMode =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceSetMode',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceSetMode);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceLocationRequest,
 *   !proto.proto.api.v1.NavigationServiceLocationResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceLocation = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceLocation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceLocationRequest,
  proto.proto.api.v1.NavigationServiceLocationResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceLocationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceLocationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceLocationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceLocation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceLocation,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceLocationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceLocation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceLocation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceWaypointsRequest,
 *   !proto.proto.api.v1.NavigationServiceWaypointsResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceWaypoints = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceWaypoints',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceWaypointsRequest,
  proto.proto.api.v1.NavigationServiceWaypointsResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceWaypointsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceWaypointsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceWaypointsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceWaypoints =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceWaypoints',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceWaypoints,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceWaypointsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceWaypoints =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceWaypoints',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceWaypoints);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceAddWaypointRequest,
 *   !proto.proto.api.v1.NavigationServiceAddWaypointResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceAddWaypoint = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceAddWaypoint',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceAddWaypointRequest,
  proto.proto.api.v1.NavigationServiceAddWaypointResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceAddWaypointResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceAddWaypointResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceAddWaypointResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceAddWaypoint =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceAddWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceAddWaypoint,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceAddWaypointResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceAddWaypoint =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceAddWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceAddWaypoint);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.NavigationServiceRemoveWaypointRequest,
 *   !proto.proto.api.v1.NavigationServiceRemoveWaypointResponse>}
 */
const methodDescriptor_RobotService_NavigationServiceRemoveWaypoint = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/NavigationServiceRemoveWaypoint',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.NavigationServiceRemoveWaypointRequest,
  proto.proto.api.v1.NavigationServiceRemoveWaypointResponse,
  /**
   * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.NavigationServiceRemoveWaypointResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.navigationServiceRemoveWaypoint =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceRemoveWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceRemoveWaypoint,
      callback);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.navigationServiceRemoveWaypoint =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/NavigationServiceRemoveWaypoint',
      request,
      metadata || {},
      methodDescriptor_RobotService_NavigationServiceRemoveWaypoint);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.IMUAngularVelocityRequest,
 *   !proto.proto.api.v1.IMUAngularVelocityResponse>}
 */
const methodDescriptor_RobotService_IMUAngularVelocity = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/IMUAngularVelocity',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.IMUAngularVelocityRequest,
  proto.proto.api.v1.IMUAngularVelocityResponse,
  /**
   * @param {!proto.proto.api.v1.IMUAngularVelocityRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.IMUAngularVelocityResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.IMUAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.IMUAngularVelocityResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.IMUAngularVelocityResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.iMUAngularVelocity =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/IMUAngularVelocity',
      request,
      metadata || {},
      methodDescriptor_RobotService_IMUAngularVelocity,
      callback);
};


/**
 * @param {!proto.proto.api.v1.IMUAngularVelocityRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.IMUAngularVelocityResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.iMUAngularVelocity =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/IMUAngularVelocity',
      request,
      metadata || {},
      methodDescriptor_RobotService_IMUAngularVelocity);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.IMUOrientationRequest,
 *   !proto.proto.api.v1.IMUOrientationResponse>}
 */
const methodDescriptor_RobotService_IMUOrientation = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/IMUOrientation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.IMUOrientationRequest,
  proto.proto.api.v1.IMUOrientationResponse,
  /**
   * @param {!proto.proto.api.v1.IMUOrientationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.IMUOrientationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.IMUOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.IMUOrientationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.IMUOrientationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.iMUOrientation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/IMUOrientation',
      request,
      metadata || {},
      methodDescriptor_RobotService_IMUOrientation,
      callback);
};


/**
 * @param {!proto.proto.api.v1.IMUOrientationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.IMUOrientationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.iMUOrientation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/IMUOrientation',
      request,
      metadata || {},
      methodDescriptor_RobotService_IMUOrientation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSLocationRequest,
 *   !proto.proto.api.v1.GPSLocationResponse>}
 */
const methodDescriptor_RobotService_GPSLocation = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSLocation',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSLocationRequest,
  proto.proto.api.v1.GPSLocationResponse,
  /**
   * @param {!proto.proto.api.v1.GPSLocationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSLocationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSLocationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSLocationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSLocation =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSLocation,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSLocationRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSLocationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSLocation =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSLocation',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSLocation);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSAltitudeRequest,
 *   !proto.proto.api.v1.GPSAltitudeResponse>}
 */
const methodDescriptor_RobotService_GPSAltitude = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSAltitude',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSAltitudeRequest,
  proto.proto.api.v1.GPSAltitudeResponse,
  /**
   * @param {!proto.proto.api.v1.GPSAltitudeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSAltitudeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSAltitudeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSAltitudeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSAltitudeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSAltitude =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAltitude',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAltitude,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSAltitudeRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSAltitudeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSAltitude =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAltitude',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAltitude);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSSpeedRequest,
 *   !proto.proto.api.v1.GPSSpeedResponse>}
 */
const methodDescriptor_RobotService_GPSSpeed = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSSpeed',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSSpeedRequest,
  proto.proto.api.v1.GPSSpeedResponse,
  /**
   * @param {!proto.proto.api.v1.GPSSpeedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSSpeedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSSpeedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSSpeedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSSpeedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSSpeed =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSSpeed',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSSpeed,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSSpeedRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSSpeedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSSpeed =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSSpeed',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSSpeed);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.GPSAccuracyRequest,
 *   !proto.proto.api.v1.GPSAccuracyResponse>}
 */
const methodDescriptor_RobotService_GPSAccuracy = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/GPSAccuracy',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.GPSAccuracyRequest,
  proto.proto.api.v1.GPSAccuracyResponse,
  /**
   * @param {!proto.proto.api.v1.GPSAccuracyRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.GPSAccuracyResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.GPSAccuracyRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.GPSAccuracyResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.GPSAccuracyResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.gPSAccuracy =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAccuracy',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAccuracy,
      callback);
};


/**
 * @param {!proto.proto.api.v1.GPSAccuracyRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.GPSAccuracyResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.gPSAccuracy =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/GPSAccuracy',
      request,
      metadata || {},
      methodDescriptor_RobotService_GPSAccuracy);
};


module.exports = proto.proto.api.v1;

