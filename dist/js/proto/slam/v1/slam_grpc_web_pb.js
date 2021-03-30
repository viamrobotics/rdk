/**
 * @fileoverview gRPC-Web generated client stub for proto.slam.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_annotations_pb = require('../../../google/api/annotations_pb.js')
const proto = {};
proto.proto = {};
proto.proto.slam = {};
proto.proto.slam.v1 = require('./slam_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.slam.v1.SlamServiceClient =
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
proto.proto.slam.v1.SlamServicePromiseClient =
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
 *   !proto.proto.slam.v1.SaveRequest,
 *   !proto.proto.slam.v1.SaveResponse>}
 */
const methodDescriptor_SlamService_Save = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/Save',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.SaveRequest,
  proto.proto.slam.v1.SaveResponse,
  /**
   * @param {!proto.proto.slam.v1.SaveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SaveResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.SaveRequest,
 *   !proto.proto.slam.v1.SaveResponse>}
 */
const methodInfo_SlamService_Save = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.SaveResponse,
  /**
   * @param {!proto.proto.slam.v1.SaveRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SaveResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.SaveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.SaveResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.SaveResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.save =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/Save',
      request,
      metadata || {},
      methodDescriptor_SlamService_Save,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.SaveRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.SaveResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.save =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/Save',
      request,
      metadata || {},
      methodDescriptor_SlamService_Save);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.StatsRequest,
 *   !proto.proto.slam.v1.StatsResponse>}
 */
const methodDescriptor_SlamService_Stats = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/Stats',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.StatsRequest,
  proto.proto.slam.v1.StatsResponse,
  /**
   * @param {!proto.proto.slam.v1.StatsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.StatsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.StatsRequest,
 *   !proto.proto.slam.v1.StatsResponse>}
 */
const methodInfo_SlamService_Stats = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.StatsResponse,
  /**
   * @param {!proto.proto.slam.v1.StatsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.StatsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.StatsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.StatsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.StatsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.stats =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/Stats',
      request,
      metadata || {},
      methodDescriptor_SlamService_Stats,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.StatsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.StatsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.stats =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/Stats',
      request,
      metadata || {},
      methodDescriptor_SlamService_Stats);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.CalibrateRequest,
 *   !proto.proto.slam.v1.CalibrateResponse>}
 */
const methodDescriptor_SlamService_Calibrate = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/Calibrate',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.CalibrateRequest,
  proto.proto.slam.v1.CalibrateResponse,
  /**
   * @param {!proto.proto.slam.v1.CalibrateRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.CalibrateResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.CalibrateRequest,
 *   !proto.proto.slam.v1.CalibrateResponse>}
 */
const methodInfo_SlamService_Calibrate = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.CalibrateResponse,
  /**
   * @param {!proto.proto.slam.v1.CalibrateRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.CalibrateResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.CalibrateRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.CalibrateResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.CalibrateResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.calibrate =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/Calibrate',
      request,
      metadata || {},
      methodDescriptor_SlamService_Calibrate,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.CalibrateRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.CalibrateResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.calibrate =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/Calibrate',
      request,
      metadata || {},
      methodDescriptor_SlamService_Calibrate);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.MoveRobotRequest,
 *   !proto.proto.slam.v1.MoveRobotResponse>}
 */
const methodDescriptor_SlamService_MoveRobot = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/MoveRobot',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.MoveRobotRequest,
  proto.proto.slam.v1.MoveRobotResponse,
  /**
   * @param {!proto.proto.slam.v1.MoveRobotRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.MoveRobotResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.MoveRobotRequest,
 *   !proto.proto.slam.v1.MoveRobotResponse>}
 */
const methodInfo_SlamService_MoveRobot = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.MoveRobotResponse,
  /**
   * @param {!proto.proto.slam.v1.MoveRobotRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.MoveRobotResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.MoveRobotRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.MoveRobotResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.MoveRobotResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.moveRobot =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/MoveRobot',
      request,
      metadata || {},
      methodDescriptor_SlamService_MoveRobot,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.MoveRobotRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.MoveRobotResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.moveRobot =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/MoveRobot',
      request,
      metadata || {},
      methodDescriptor_SlamService_MoveRobot);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.MoveRobotForwardRequest,
 *   !proto.proto.slam.v1.MoveRobotForwardResponse>}
 */
const methodDescriptor_SlamService_MoveRobotForward = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/MoveRobotForward',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.MoveRobotForwardRequest,
  proto.proto.slam.v1.MoveRobotForwardResponse,
  /**
   * @param {!proto.proto.slam.v1.MoveRobotForwardRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.MoveRobotForwardResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.MoveRobotForwardRequest,
 *   !proto.proto.slam.v1.MoveRobotForwardResponse>}
 */
const methodInfo_SlamService_MoveRobotForward = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.MoveRobotForwardResponse,
  /**
   * @param {!proto.proto.slam.v1.MoveRobotForwardRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.MoveRobotForwardResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.MoveRobotForwardRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.MoveRobotForwardResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.MoveRobotForwardResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.moveRobotForward =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/MoveRobotForward',
      request,
      metadata || {},
      methodDescriptor_SlamService_MoveRobotForward,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.MoveRobotForwardRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.MoveRobotForwardResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.moveRobotForward =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/MoveRobotForward',
      request,
      metadata || {},
      methodDescriptor_SlamService_MoveRobotForward);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.MoveRobotBackwardRequest,
 *   !proto.proto.slam.v1.MoveRobotBackwardResponse>}
 */
const methodDescriptor_SlamService_MoveRobotBackward = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/MoveRobotBackward',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.MoveRobotBackwardRequest,
  proto.proto.slam.v1.MoveRobotBackwardResponse,
  /**
   * @param {!proto.proto.slam.v1.MoveRobotBackwardRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.MoveRobotBackwardResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.MoveRobotBackwardRequest,
 *   !proto.proto.slam.v1.MoveRobotBackwardResponse>}
 */
const methodInfo_SlamService_MoveRobotBackward = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.MoveRobotBackwardResponse,
  /**
   * @param {!proto.proto.slam.v1.MoveRobotBackwardRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.MoveRobotBackwardResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.MoveRobotBackwardRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.MoveRobotBackwardResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.MoveRobotBackwardResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.moveRobotBackward =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/MoveRobotBackward',
      request,
      metadata || {},
      methodDescriptor_SlamService_MoveRobotBackward,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.MoveRobotBackwardRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.MoveRobotBackwardResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.moveRobotBackward =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/MoveRobotBackward',
      request,
      metadata || {},
      methodDescriptor_SlamService_MoveRobotBackward);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.TurnRobotToRequest,
 *   !proto.proto.slam.v1.TurnRobotToResponse>}
 */
const methodDescriptor_SlamService_TurnRobotTo = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/TurnRobotTo',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.TurnRobotToRequest,
  proto.proto.slam.v1.TurnRobotToResponse,
  /**
   * @param {!proto.proto.slam.v1.TurnRobotToRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.TurnRobotToResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.TurnRobotToRequest,
 *   !proto.proto.slam.v1.TurnRobotToResponse>}
 */
const methodInfo_SlamService_TurnRobotTo = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.TurnRobotToResponse,
  /**
   * @param {!proto.proto.slam.v1.TurnRobotToRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.TurnRobotToResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.TurnRobotToRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.TurnRobotToResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.TurnRobotToResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.turnRobotTo =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/TurnRobotTo',
      request,
      metadata || {},
      methodDescriptor_SlamService_TurnRobotTo,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.TurnRobotToRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.TurnRobotToResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.turnRobotTo =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/TurnRobotTo',
      request,
      metadata || {},
      methodDescriptor_SlamService_TurnRobotTo);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.UpdateRobotDeviceOffsetRequest,
 *   !proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse>}
 */
const methodDescriptor_SlamService_UpdateRobotDeviceOffset = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/UpdateRobotDeviceOffset',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.UpdateRobotDeviceOffsetRequest,
  proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse,
  /**
   * @param {!proto.proto.slam.v1.UpdateRobotDeviceOffsetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.UpdateRobotDeviceOffsetRequest,
 *   !proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse>}
 */
const methodInfo_SlamService_UpdateRobotDeviceOffset = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse,
  /**
   * @param {!proto.proto.slam.v1.UpdateRobotDeviceOffsetRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.UpdateRobotDeviceOffsetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.updateRobotDeviceOffset =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/UpdateRobotDeviceOffset',
      request,
      metadata || {},
      methodDescriptor_SlamService_UpdateRobotDeviceOffset,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.UpdateRobotDeviceOffsetRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.UpdateRobotDeviceOffsetResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.updateRobotDeviceOffset =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/UpdateRobotDeviceOffset',
      request,
      metadata || {},
      methodDescriptor_SlamService_UpdateRobotDeviceOffset);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.StartLidarRequest,
 *   !proto.proto.slam.v1.StartLidarResponse>}
 */
const methodDescriptor_SlamService_StartLidar = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/StartLidar',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.StartLidarRequest,
  proto.proto.slam.v1.StartLidarResponse,
  /**
   * @param {!proto.proto.slam.v1.StartLidarRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.StartLidarResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.StartLidarRequest,
 *   !proto.proto.slam.v1.StartLidarResponse>}
 */
const methodInfo_SlamService_StartLidar = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.StartLidarResponse,
  /**
   * @param {!proto.proto.slam.v1.StartLidarRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.StartLidarResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.StartLidarRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.StartLidarResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.StartLidarResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.startLidar =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/StartLidar',
      request,
      metadata || {},
      methodDescriptor_SlamService_StartLidar,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.StartLidarRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.StartLidarResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.startLidar =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/StartLidar',
      request,
      metadata || {},
      methodDescriptor_SlamService_StartLidar);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.StopLidarRequest,
 *   !proto.proto.slam.v1.StopLidarResponse>}
 */
const methodDescriptor_SlamService_StopLidar = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/StopLidar',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.StopLidarRequest,
  proto.proto.slam.v1.StopLidarResponse,
  /**
   * @param {!proto.proto.slam.v1.StopLidarRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.StopLidarResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.StopLidarRequest,
 *   !proto.proto.slam.v1.StopLidarResponse>}
 */
const methodInfo_SlamService_StopLidar = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.StopLidarResponse,
  /**
   * @param {!proto.proto.slam.v1.StopLidarRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.StopLidarResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.StopLidarRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.StopLidarResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.StopLidarResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.stopLidar =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/StopLidar',
      request,
      metadata || {},
      methodDescriptor_SlamService_StopLidar,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.StopLidarRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.StopLidarResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.stopLidar =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/StopLidar',
      request,
      metadata || {},
      methodDescriptor_SlamService_StopLidar);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.GetLidarSeedRequest,
 *   !proto.proto.slam.v1.GetLidarSeedResponse>}
 */
const methodDescriptor_SlamService_GetLidarSeed = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/GetLidarSeed',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.GetLidarSeedRequest,
  proto.proto.slam.v1.GetLidarSeedResponse,
  /**
   * @param {!proto.proto.slam.v1.GetLidarSeedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.GetLidarSeedResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.GetLidarSeedRequest,
 *   !proto.proto.slam.v1.GetLidarSeedResponse>}
 */
const methodInfo_SlamService_GetLidarSeed = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.GetLidarSeedResponse,
  /**
   * @param {!proto.proto.slam.v1.GetLidarSeedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.GetLidarSeedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.GetLidarSeedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.GetLidarSeedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.GetLidarSeedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.getLidarSeed =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/GetLidarSeed',
      request,
      metadata || {},
      methodDescriptor_SlamService_GetLidarSeed,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.GetLidarSeedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.GetLidarSeedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.getLidarSeed =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/GetLidarSeed',
      request,
      metadata || {},
      methodDescriptor_SlamService_GetLidarSeed);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.SetLidarSeedRequest,
 *   !proto.proto.slam.v1.SetLidarSeedResponse>}
 */
const methodDescriptor_SlamService_SetLidarSeed = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/SetLidarSeed',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.SetLidarSeedRequest,
  proto.proto.slam.v1.SetLidarSeedResponse,
  /**
   * @param {!proto.proto.slam.v1.SetLidarSeedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetLidarSeedResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.SetLidarSeedRequest,
 *   !proto.proto.slam.v1.SetLidarSeedResponse>}
 */
const methodInfo_SlamService_SetLidarSeed = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.SetLidarSeedResponse,
  /**
   * @param {!proto.proto.slam.v1.SetLidarSeedRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetLidarSeedResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.SetLidarSeedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.SetLidarSeedResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.SetLidarSeedResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.setLidarSeed =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetLidarSeed',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetLidarSeed,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.SetLidarSeedRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.SetLidarSeedResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.setLidarSeed =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetLidarSeed',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetLidarSeed);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.SetClientZoomRequest,
 *   !proto.proto.slam.v1.SetClientZoomResponse>}
 */
const methodDescriptor_SlamService_SetClientZoom = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/SetClientZoom',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.SetClientZoomRequest,
  proto.proto.slam.v1.SetClientZoomResponse,
  /**
   * @param {!proto.proto.slam.v1.SetClientZoomRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetClientZoomResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.SetClientZoomRequest,
 *   !proto.proto.slam.v1.SetClientZoomResponse>}
 */
const methodInfo_SlamService_SetClientZoom = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.SetClientZoomResponse,
  /**
   * @param {!proto.proto.slam.v1.SetClientZoomRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetClientZoomResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.SetClientZoomRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.SetClientZoomResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.SetClientZoomResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.setClientZoom =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetClientZoom',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetClientZoom,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.SetClientZoomRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.SetClientZoomResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.setClientZoom =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetClientZoom',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetClientZoom);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.SetClientLidarViewModeRequest,
 *   !proto.proto.slam.v1.SetClientLidarViewModeResponse>}
 */
const methodDescriptor_SlamService_SetClientLidarViewMode = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/SetClientLidarViewMode',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.SetClientLidarViewModeRequest,
  proto.proto.slam.v1.SetClientLidarViewModeResponse,
  /**
   * @param {!proto.proto.slam.v1.SetClientLidarViewModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetClientLidarViewModeResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.SetClientLidarViewModeRequest,
 *   !proto.proto.slam.v1.SetClientLidarViewModeResponse>}
 */
const methodInfo_SlamService_SetClientLidarViewMode = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.SetClientLidarViewModeResponse,
  /**
   * @param {!proto.proto.slam.v1.SetClientLidarViewModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetClientLidarViewModeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.SetClientLidarViewModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.SetClientLidarViewModeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.SetClientLidarViewModeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.setClientLidarViewMode =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetClientLidarViewMode',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetClientLidarViewMode,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.SetClientLidarViewModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.SetClientLidarViewModeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.setClientLidarViewMode =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetClientLidarViewMode',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetClientLidarViewMode);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.slam.v1.SetClientClickModeRequest,
 *   !proto.proto.slam.v1.SetClientClickModeResponse>}
 */
const methodDescriptor_SlamService_SetClientClickMode = new grpc.web.MethodDescriptor(
  '/proto.slam.v1.SlamService/SetClientClickMode',
  grpc.web.MethodType.UNARY,
  proto.proto.slam.v1.SetClientClickModeRequest,
  proto.proto.slam.v1.SetClientClickModeResponse,
  /**
   * @param {!proto.proto.slam.v1.SetClientClickModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetClientClickModeResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.slam.v1.SetClientClickModeRequest,
 *   !proto.proto.slam.v1.SetClientClickModeResponse>}
 */
const methodInfo_SlamService_SetClientClickMode = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.slam.v1.SetClientClickModeResponse,
  /**
   * @param {!proto.proto.slam.v1.SetClientClickModeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.slam.v1.SetClientClickModeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.slam.v1.SetClientClickModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.slam.v1.SetClientClickModeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.slam.v1.SetClientClickModeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.slam.v1.SlamServiceClient.prototype.setClientClickMode =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetClientClickMode',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetClientClickMode,
      callback);
};


/**
 * @param {!proto.proto.slam.v1.SetClientClickModeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.slam.v1.SetClientClickModeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.slam.v1.SlamServicePromiseClient.prototype.setClientClickMode =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.slam.v1.SlamService/SetClientClickMode',
      request,
      metadata || {},
      methodDescriptor_SlamService_SetClientClickMode);
};


module.exports = proto.proto.slam.v1;

