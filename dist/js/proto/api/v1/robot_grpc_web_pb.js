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
 *   !proto.proto.api.v1.ControlBaseRequest,
 *   !proto.proto.api.v1.ControlBaseResponse>}
 */
const methodDescriptor_RobotService_ControlBase = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ControlBase',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ControlBaseRequest,
  proto.proto.api.v1.ControlBaseResponse,
  /**
   * @param {!proto.proto.api.v1.ControlBaseRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlBaseResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ControlBaseRequest,
 *   !proto.proto.api.v1.ControlBaseResponse>}
 */
const methodInfo_RobotService_ControlBase = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.ControlBaseResponse,
  /**
   * @param {!proto.proto.api.v1.ControlBaseRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlBaseResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ControlBaseRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ControlBaseResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ControlBaseResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.controlBase =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlBase',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlBase,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ControlBaseRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ControlBaseResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.controlBase =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlBase',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlBase);
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
 *   !proto.proto.api.v1.MoveArmToPositionRequest,
 *   !proto.proto.api.v1.MoveArmToPositionResponse>}
 */
const methodDescriptor_RobotService_MoveArmToPosition = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MoveArmToPosition',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MoveArmToPositionRequest,
  proto.proto.api.v1.MoveArmToPositionResponse,
  /**
   * @param {!proto.proto.api.v1.MoveArmToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MoveArmToPositionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.MoveArmToPositionRequest,
 *   !proto.proto.api.v1.MoveArmToPositionResponse>}
 */
const methodInfo_RobotService_MoveArmToPosition = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.MoveArmToPositionResponse,
  /**
   * @param {!proto.proto.api.v1.MoveArmToPositionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MoveArmToPositionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MoveArmToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.MoveArmToPositionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MoveArmToPositionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.moveArmToPosition =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MoveArmToPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_MoveArmToPosition,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MoveArmToPositionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MoveArmToPositionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.moveArmToPosition =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MoveArmToPosition',
      request,
      metadata || {},
      methodDescriptor_RobotService_MoveArmToPosition);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.MoveArmToJointPositionsRequest,
 *   !proto.proto.api.v1.MoveArmToJointPositionsResponse>}
 */
const methodDescriptor_RobotService_MoveArmToJointPositions = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/MoveArmToJointPositions',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.MoveArmToJointPositionsRequest,
  proto.proto.api.v1.MoveArmToJointPositionsResponse,
  /**
   * @param {!proto.proto.api.v1.MoveArmToJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MoveArmToJointPositionsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.MoveArmToJointPositionsRequest,
 *   !proto.proto.api.v1.MoveArmToJointPositionsResponse>}
 */
const methodInfo_RobotService_MoveArmToJointPositions = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.MoveArmToJointPositionsResponse,
  /**
   * @param {!proto.proto.api.v1.MoveArmToJointPositionsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.MoveArmToJointPositionsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.MoveArmToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.MoveArmToJointPositionsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.MoveArmToJointPositionsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.moveArmToJointPositions =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/MoveArmToJointPositions',
      request,
      metadata || {},
      methodDescriptor_RobotService_MoveArmToJointPositions,
      callback);
};


/**
 * @param {!proto.proto.api.v1.MoveArmToJointPositionsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.MoveArmToJointPositionsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.moveArmToJointPositions =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/MoveArmToJointPositions',
      request,
      metadata || {},
      methodDescriptor_RobotService_MoveArmToJointPositions);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ControlGripperRequest,
 *   !proto.proto.api.v1.ControlGripperResponse>}
 */
const methodDescriptor_RobotService_ControlGripper = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ControlGripper',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ControlGripperRequest,
  proto.proto.api.v1.ControlGripperResponse,
  /**
   * @param {!proto.proto.api.v1.ControlGripperRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlGripperResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ControlGripperRequest,
 *   !proto.proto.api.v1.ControlGripperResponse>}
 */
const methodInfo_RobotService_ControlGripper = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.ControlGripperResponse,
  /**
   * @param {!proto.proto.api.v1.ControlGripperRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlGripperResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ControlGripperRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ControlGripperResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ControlGripperResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.controlGripper =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlGripper',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlGripper,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ControlGripperRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ControlGripperResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.controlGripper =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlGripper',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlGripper);
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
 *   !proto.proto.api.v1.ControlBoardMotorRequest,
 *   !proto.proto.api.v1.ControlBoardMotorResponse>}
 */
const methodDescriptor_RobotService_ControlBoardMotor = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ControlBoardMotor',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ControlBoardMotorRequest,
  proto.proto.api.v1.ControlBoardMotorResponse,
  /**
   * @param {!proto.proto.api.v1.ControlBoardMotorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlBoardMotorResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ControlBoardMotorRequest,
 *   !proto.proto.api.v1.ControlBoardMotorResponse>}
 */
const methodInfo_RobotService_ControlBoardMotor = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.ControlBoardMotorResponse,
  /**
   * @param {!proto.proto.api.v1.ControlBoardMotorRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlBoardMotorResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ControlBoardMotorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ControlBoardMotorResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ControlBoardMotorResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.controlBoardMotor =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlBoardMotor',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlBoardMotor,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ControlBoardMotorRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ControlBoardMotorResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.controlBoardMotor =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlBoardMotor',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlBoardMotor);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.v1.ControlBoardServoRequest,
 *   !proto.proto.api.v1.ControlBoardServoResponse>}
 */
const methodDescriptor_RobotService_ControlBoardServo = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ControlBoardServo',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ControlBoardServoRequest,
  proto.proto.api.v1.ControlBoardServoResponse,
  /**
   * @param {!proto.proto.api.v1.ControlBoardServoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlBoardServoResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.v1.ControlBoardServoRequest,
 *   !proto.proto.api.v1.ControlBoardServoResponse>}
 */
const methodInfo_RobotService_ControlBoardServo = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.v1.ControlBoardServoResponse,
  /**
   * @param {!proto.proto.api.v1.ControlBoardServoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ControlBoardServoResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ControlBoardServoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.v1.ControlBoardServoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ControlBoardServoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.controlBoardServo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlBoardServo',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlBoardServo,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ControlBoardServoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ControlBoardServoResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.controlBoardServo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ControlBoardServo',
      request,
      metadata || {},
      methodDescriptor_RobotService_ControlBoardServo);
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
 *   !proto.proto.api.v1.CameraFrameRequest,
 *   !proto.google.api.HttpBody>}
 */
const methodDescriptor_RobotService_RenderCameraFrame = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/RenderCameraFrame',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.CameraFrameRequest,
  google_api_httpbody_pb.HttpBody,
  /**
   * @param {!proto.proto.api.v1.CameraFrameRequest} request
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
 *   !proto.proto.api.v1.CameraFrameRequest,
 *   !proto.google.api.HttpBody>}
 */
const methodInfo_RobotService_RenderCameraFrame = new grpc.web.AbstractClientBase.MethodInfo(
  google_api_httpbody_pb.HttpBody,
  /**
   * @param {!proto.proto.api.v1.CameraFrameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_httpbody_pb.HttpBody.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.CameraFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.google.api.HttpBody)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.HttpBody>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.renderCameraFrame =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/RenderCameraFrame',
      request,
      metadata || {},
      methodDescriptor_RobotService_RenderCameraFrame,
      callback);
};


/**
 * @param {!proto.proto.api.v1.CameraFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.HttpBody>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.renderCameraFrame =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/RenderCameraFrame',
      request,
      metadata || {},
      methodDescriptor_RobotService_RenderCameraFrame);
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


module.exports = proto.proto.api.v1;

