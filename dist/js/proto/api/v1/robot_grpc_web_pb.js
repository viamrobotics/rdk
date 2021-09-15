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

var google_api_annotations_pb = require('../../../google/api/annotations_pb.js')

var google_api_httpbody_pb = require('../../../google/api/httpbody_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.v1 = require('./robot_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.v1.RobotServiceClient =
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
proto.proto.api.v1.RobotServicePromiseClient =
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.StatusRequest,
 *   !proto.proto.api.v1.StatusResponse>}
 */
const methodInfo_RobotService_Status = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.StatusResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.StatusStreamRequest,
 *   !proto.proto.api.v1.StatusStreamResponse>}
 */
const methodInfo_RobotService_StatusStream = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {?Object<string, string>} metadata User defined
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ConfigRequest,
 *   !proto.proto.api.v1.ConfigResponse>}
 */
const methodInfo_RobotService_Config = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ConfigResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.DoActionRequest,
 *   !proto.proto.api.v1.DoActionResponse>}
 */
const methodInfo_RobotService_DoAction = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.DoActionResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ArmCurrentPositionRequest,
 *   !proto.proto.api.v1.ArmCurrentPositionResponse>}
 */
const methodInfo_RobotService_ArmCurrentPosition = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ArmCurrentPositionResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ArmMoveToPositionRequest,
 *   !proto.proto.api.v1.ArmMoveToPositionResponse>}
 */
const methodInfo_RobotService_ArmMoveToPosition = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ArmMoveToPositionResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ArmCurrentJointPositionsRequest,
 *   !proto.proto.api.v1.ArmCurrentJointPositionsResponse>}
 */
const methodInfo_RobotService_ArmCurrentJointPositions = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ArmCurrentJointPositionsResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ArmMoveToJointPositionsRequest,
 *   !proto.proto.api.v1.ArmMoveToJointPositionsResponse>}
 */
const methodInfo_RobotService_ArmMoveToJointPositions = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ArmMoveToJointPositionsResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ArmJointMoveDeltaRequest,
 *   !proto.proto.api.v1.ArmJointMoveDeltaResponse>}
 */
const methodInfo_RobotService_ArmJointMoveDelta = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ArmJointMoveDeltaResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BaseMoveStraightRequest,
 *   !proto.proto.api.v1.BaseMoveStraightResponse>}
 */
const methodInfo_RobotService_BaseMoveStraight = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BaseMoveStraightResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BaseSpinRequest,
 *   !proto.proto.api.v1.BaseSpinResponse>}
 */
const methodInfo_RobotService_BaseSpin = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BaseSpinResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BaseStopRequest,
 *   !proto.proto.api.v1.BaseStopResponse>}
 */
const methodInfo_RobotService_BaseStop = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BaseStopResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BaseWidthMillisRequest,
 *   !proto.proto.api.v1.BaseWidthMillisResponse>}
 */
const methodInfo_RobotService_BaseWidthMillis = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BaseWidthMillisResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.GripperOpenRequest,
 *   !proto.proto.api.v1.GripperOpenResponse>}
 */
const methodInfo_RobotService_GripperOpen = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.GripperOpenResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.GripperGrabRequest,
 *   !proto.proto.api.v1.GripperGrabResponse>}
 */
const methodInfo_RobotService_GripperGrab = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.GripperGrabResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.CameraFrameRequest,
 *   !proto.proto.api.v1.CameraFrameResponse>}
 */
const methodInfo_RobotService_CameraFrame = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.CameraFrameResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.CameraRenderFrameRequest,
 *   !proto.google.api.HttpBody>}
 */
const methodInfo_RobotService_CameraRenderFrame = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.google.api.HttpBody)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.PointCloudRequest,
 *   !proto.proto.api.v1.PointCloudResponse>}
 */
const methodInfo_RobotService_PointCloud = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.PointCloudResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ObjectPointCloudsRequest,
 *   !proto.proto.api.v1.ObjectPointCloudsResponse>}
 */
const methodInfo_RobotService_ObjectPointClouds = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ObjectPointCloudsResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.LidarInfoRequest,
 *   !proto.proto.api.v1.LidarInfoResponse>}
 */
const methodInfo_RobotService_LidarInfo = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.LidarInfoResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.LidarStartRequest,
 *   !proto.proto.api.v1.LidarStartResponse>}
 */
const methodInfo_RobotService_LidarStart = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.LidarStartResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.LidarStopRequest,
 *   !proto.proto.api.v1.LidarStopResponse>}
 */
const methodInfo_RobotService_LidarStop = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.LidarStopResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.LidarScanRequest,
 *   !proto.proto.api.v1.LidarScanResponse>}
 */
const methodInfo_RobotService_LidarScan = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.LidarScanResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.LidarRangeRequest,
 *   !proto.proto.api.v1.LidarRangeResponse>}
 */
const methodInfo_RobotService_LidarRange = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.LidarRangeResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.LidarBoundsRequest,
 *   !proto.proto.api.v1.LidarBoundsResponse>}
 */
const methodInfo_RobotService_LidarBounds = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.LidarBoundsResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.LidarAngularResolutionRequest,
 *   !proto.proto.api.v1.LidarAngularResolutionResponse>}
 */
const methodInfo_RobotService_LidarAngularResolution = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.LidarAngularResolutionResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardStatusRequest,
 *   !proto.proto.api.v1.BoardStatusResponse>}
 */
const methodInfo_RobotService_BoardStatus = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardStatusResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardGPIOSetRequest,
 *   !proto.proto.api.v1.BoardGPIOSetResponse>}
 */
const methodInfo_RobotService_BoardGPIOSet = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardGPIOSetResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardGPIOGetRequest,
 *   !proto.proto.api.v1.BoardGPIOGetResponse>}
 */
const methodInfo_RobotService_BoardGPIOGet = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardGPIOGetResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardPWMSetRequest,
 *   !proto.proto.api.v1.BoardPWMSetResponse>}
 */
const methodInfo_RobotService_BoardPWMSet = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardPWMSetResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardPWMSetFrequencyRequest,
 *   !proto.proto.api.v1.BoardPWMSetFrequencyResponse>}
 */
const methodInfo_RobotService_BoardPWMSetFrequency = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardPWMSetFrequencyResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 *   !proto.proto.api.v1.BoardMotorPowerRequest,
 *   !proto.proto.api.v1.BoardMotorPowerResponse>}
 */
const methodDescriptor_RobotService_BoardMotorPower = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorPower',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorPowerRequest,
  proto.proto.api.v1.BoardMotorPowerResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorPowerRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorPowerResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorPowerRequest,
 *   !proto.proto.api.v1.BoardMotorPowerResponse>}
 */
const methodInfo_RobotService_BoardMotorPower = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorPowerResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorPowerRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorPowerResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorPowerRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorPowerResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorPowerResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorPower =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorPower',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorPower,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorPowerRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorPowerResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorPower =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorPower',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorPower);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorGoRequest,
 *   !proto.proto.api.v1.BoardMotorGoResponse>}
 */
const methodDescriptor_RobotService_BoardMotorGo = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorGo',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorGoRequest,
  proto.proto.api.v1.BoardMotorGoResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorGoRequest,
 *   !proto.proto.api.v1.BoardMotorGoResponse>}
 */
const methodInfo_RobotService_BoardMotorGo = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorGoResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorGoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorGoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorGoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorGo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGo',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGo,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorGoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorGoResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorGo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGo',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorGoForRequest,
 *   !proto.proto.api.v1.BoardMotorGoForResponse>}
 */
const methodDescriptor_RobotService_BoardMotorGoFor = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorGoFor',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorGoForRequest,
  proto.proto.api.v1.BoardMotorGoForResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoForRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoForResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorGoForRequest,
 *   !proto.proto.api.v1.BoardMotorGoForResponse>}
 */
const methodInfo_RobotService_BoardMotorGoFor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorGoForResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoForRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoForResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorGoForRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorGoForResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorGoForResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorGoFor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGoFor',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGoFor,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorGoForRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorGoForResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorGoFor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGoFor',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGoFor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorGoToRequest,
 *   !proto.proto.api.v1.BoardMotorGoToResponse>}
 */
const methodDescriptor_RobotService_BoardMotorGoTo = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorGoTo',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorGoToRequest,
  proto.proto.api.v1.BoardMotorGoToResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoToRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoToResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorGoToRequest,
 *   !proto.proto.api.v1.BoardMotorGoToResponse>}
 */
const methodInfo_RobotService_BoardMotorGoTo = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorGoToResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoToRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoToResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorGoToRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorGoToResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorGoToResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorGoTo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGoTo',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGoTo,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorGoToRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorGoToResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorGoTo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGoTo',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGoTo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorGoTillStopRequest,
 *   !proto.proto.api.v1.BoardMotorGoTillStopResponse>}
 */
const methodDescriptor_RobotService_BoardMotorGoTillStop = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorGoTillStop',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorGoTillStopRequest,
  proto.proto.api.v1.BoardMotorGoTillStopResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoTillStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoTillStopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorGoTillStopRequest,
 *   !proto.proto.api.v1.BoardMotorGoTillStopResponse>}
 */
const methodInfo_RobotService_BoardMotorGoTillStop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorGoTillStopResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorGoTillStopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorGoTillStopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorGoTillStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorGoTillStopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorGoTillStopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorGoTillStop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGoTillStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGoTillStop,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorGoTillStopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorGoTillStopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorGoTillStop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorGoTillStop',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorGoTillStop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorZeroRequest,
 *   !proto.proto.api.v1.BoardMotorZeroResponse>}
 */
const methodDescriptor_RobotService_BoardMotorZero = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorZero',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorZeroRequest,
  proto.proto.api.v1.BoardMotorZeroResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorZeroRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorZeroResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorZeroRequest,
 *   !proto.proto.api.v1.BoardMotorZeroResponse>}
 */
const methodInfo_RobotService_BoardMotorZero = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorZeroResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorZeroRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorZeroResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorZeroRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorZeroResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorZeroResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorZero =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorZero',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorZero,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorZeroRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorZeroResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorZero =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorZero',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorZero);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorPositionRequest,
 *   !proto.proto.api.v1.BoardMotorPositionResponse>}
 */
const methodDescriptor_RobotService_BoardMotorPosition = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorPositionRequest,
  proto.proto.api.v1.BoardMotorPositionResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorPositionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorPositionRequest,
 *   !proto.proto.api.v1.BoardMotorPositionResponse>}
 */
const methodInfo_RobotService_BoardMotorPosition = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorPositionResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorPosition,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorPositionSupportedRequest,
 *   !proto.proto.api.v1.BoardMotorPositionSupportedResponse>}
 */
const methodDescriptor_RobotService_BoardMotorPositionSupported = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorPositionSupported',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorPositionSupportedRequest,
  proto.proto.api.v1.BoardMotorPositionSupportedResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorPositionSupportedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorPositionSupportedResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorPositionSupportedRequest,
 *   !proto.proto.api.v1.BoardMotorPositionSupportedResponse>}
 */
const methodInfo_RobotService_BoardMotorPositionSupported = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorPositionSupportedResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorPositionSupportedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorPositionSupportedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorPositionSupportedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorPositionSupportedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorPositionSupportedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorPositionSupported =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorPositionSupported',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorPositionSupported,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorPositionSupportedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorPositionSupportedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorPositionSupported =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorPositionSupported',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorPositionSupported);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorOffRequest,
 *   !proto.proto.api.v1.BoardMotorOffResponse>}
 */
const methodDescriptor_RobotService_BoardMotorOff = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorOff',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorOffRequest,
  proto.proto.api.v1.BoardMotorOffResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorOffRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorOffResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorOffRequest,
 *   !proto.proto.api.v1.BoardMotorOffResponse>}
 */
const methodInfo_RobotService_BoardMotorOff = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorOffResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorOffRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorOffResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorOffRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorOffResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorOffResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorOff =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorOff',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorOff,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorOffRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorOffResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorOff =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorOff',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorOff);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardMotorIsOnRequest,
 *   !proto.proto.api.v1.BoardMotorIsOnResponse>}
 */
const methodDescriptor_RobotService_BoardMotorIsOn = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardMotorIsOn',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardMotorIsOnRequest,
  proto.proto.api.v1.BoardMotorIsOnResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorIsOnRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorIsOnResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardMotorIsOnRequest,
 *   !proto.proto.api.v1.BoardMotorIsOnResponse>}
 */
const methodInfo_RobotService_BoardMotorIsOn = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardMotorIsOnResponse,
  /**
   * @param {!proto.proto.api.v1.BoardMotorIsOnRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardMotorIsOnResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardMotorIsOnRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardMotorIsOnResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardMotorIsOnResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardMotorIsOn =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorIsOn',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorIsOn,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardMotorIsOnRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardMotorIsOnResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardMotorIsOn =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardMotorIsOn',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardMotorIsOn);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardServoMoveRequest,
 *   !proto.proto.api.v1.BoardServoMoveResponse>}
 */
const methodDescriptor_RobotService_BoardServoMove = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardServoMove',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardServoMoveRequest,
  proto.proto.api.v1.BoardServoMoveResponse,
  /**
   * @param {!proto.proto.api.v1.BoardServoMoveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardServoMoveResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardServoMoveRequest,
 *   !proto.proto.api.v1.BoardServoMoveResponse>}
 */
const methodInfo_RobotService_BoardServoMove = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardServoMoveResponse,
  /**
   * @param {!proto.proto.api.v1.BoardServoMoveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardServoMoveResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardServoMoveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardServoMoveResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardServoMoveResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardServoMove =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardServoMove',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardServoMove,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardServoMoveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardServoMoveResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardServoMove =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardServoMove',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardServoMove);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.BoardServoCurrentRequest,
 *   !proto.proto.api.v1.BoardServoCurrentResponse>}
 */
const methodDescriptor_RobotService_BoardServoCurrent = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/BoardServoCurrent',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.BoardServoCurrentRequest,
  proto.proto.api.v1.BoardServoCurrentResponse,
  /**
   * @param {!proto.proto.api.v1.BoardServoCurrentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardServoCurrentResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardServoCurrentRequest,
 *   !proto.proto.api.v1.BoardServoCurrentResponse>}
 */
const methodInfo_RobotService_BoardServoCurrent = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.BoardServoCurrentResponse,
  /**
   * @param {!proto.proto.api.v1.BoardServoCurrentRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.BoardServoCurrentResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.BoardServoCurrentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardServoCurrentResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.BoardServoCurrentResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.boardServoCurrent =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardServoCurrent',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardServoCurrent,
      callback);
};


/**
 * @param {!proto.proto.api.v1.BoardServoCurrentRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.BoardServoCurrentResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.boardServoCurrent =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/BoardServoCurrent',
      request,
      metadata || {},
      methodDescriptor_RobotService_BoardServoCurrent);
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardAnalogReaderReadRequest,
 *   !proto.proto.api.v1.BoardAnalogReaderReadResponse>}
 */
const methodInfo_RobotService_BoardAnalogReaderRead = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardAnalogReaderReadResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardDigitalInterruptConfigRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptConfigResponse>}
 */
const methodInfo_RobotService_BoardDigitalInterruptConfig = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardDigitalInterruptConfigResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardDigitalInterruptValueRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptValueResponse>}
 */
const methodInfo_RobotService_BoardDigitalInterruptValue = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardDigitalInterruptValueResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.BoardDigitalInterruptTickRequest,
 *   !proto.proto.api.v1.BoardDigitalInterruptTickResponse>}
 */
const methodInfo_RobotService_BoardDigitalInterruptTick = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.BoardDigitalInterruptTickResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.SensorReadingsRequest,
 *   !proto.proto.api.v1.SensorReadingsResponse>}
 */
const methodInfo_RobotService_SensorReadings = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.SensorReadingsResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.CompassHeadingRequest,
 *   !proto.proto.api.v1.CompassHeadingResponse>}
 */
const methodInfo_RobotService_CompassHeading = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.CompassHeadingResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.CompassStartCalibrationRequest,
 *   !proto.proto.api.v1.CompassStartCalibrationResponse>}
 */
const methodInfo_RobotService_CompassStartCalibration = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.CompassStartCalibrationResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.CompassStopCalibrationRequest,
 *   !proto.proto.api.v1.CompassStopCalibrationResponse>}
 */
const methodInfo_RobotService_CompassStopCalibration = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.CompassStopCalibrationResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.CompassMarkRequest,
 *   !proto.proto.api.v1.CompassMarkResponse>}
 */
const methodInfo_RobotService_CompassMark = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.CompassMarkResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ExecuteFunctionRequest,
 *   !proto.proto.api.v1.ExecuteFunctionResponse>}
 */
const methodInfo_RobotService_ExecuteFunction = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ExecuteFunctionResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ExecuteSourceRequest,
 *   !proto.proto.api.v1.ExecuteSourceResponse>}
 */
const methodInfo_RobotService_ExecuteSource = new grpc.web.AbstractClientBase.MethodInfo(
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
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ExecuteSourceResponse)}
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
 * @param {?Object<string, string>} metadata User defined
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


module.exports = proto.proto.api.v1;

