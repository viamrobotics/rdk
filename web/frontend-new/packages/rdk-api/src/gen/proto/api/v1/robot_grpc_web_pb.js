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

var proto_api_common_v1_common_pb = require('../../../proto/api/common/v1/common_pb.js')
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
 *   !proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest,
 *   !proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse>}
 */
const methodDescriptor_RobotService_ObjectManipulationServiceDoGrab = new grpc.web.MethodDescriptor(
  '/proto.api.v1.RobotService/ObjectManipulationServiceDoGrab',
  grpc.web.MethodType.UNARY,
  proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest,
  proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse,
  /**
   * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.v1.RobotServiceClient.prototype.objectManipulationServiceDoGrab =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.v1.RobotService/ObjectManipulationServiceDoGrab',
      request,
      metadata || {},
      methodDescriptor_RobotService_ObjectManipulationServiceDoGrab,
      callback);
};


/**
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.v1.RobotServicePromiseClient.prototype.objectManipulationServiceDoGrab =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.v1.RobotService/ObjectManipulationServiceDoGrab',
      request,
      metadata || {},
      methodDescriptor_RobotService_ObjectManipulationServiceDoGrab);
};


module.exports = proto.proto.api.v1;

