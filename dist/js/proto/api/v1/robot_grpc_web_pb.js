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


module.exports = proto.proto.api.v1;

