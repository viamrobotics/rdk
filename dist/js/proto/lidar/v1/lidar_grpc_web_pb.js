/**
 * @fileoverview gRPC-Web generated client stub for proto.lidar.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js')

var google_api_annotations_pb = require('../../../google/api/annotations_pb.js')
const proto = {};
proto.proto = {};
proto.proto.lidar = {};
proto.proto.lidar.v1 = require('./lidar_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.lidar.v1.LidarServiceClient =
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
proto.proto.lidar.v1.LidarServicePromiseClient =
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
 *   !proto.proto.lidar.v1.InfoRequest,
 *   !proto.proto.lidar.v1.InfoResponse>}
 */
const methodDescriptor_LidarService_Info = new grpc.web.MethodDescriptor(
  '/proto.lidar.v1.LidarService/Info',
  grpc.web.MethodType.UNARY,
  proto.proto.lidar.v1.InfoRequest,
  proto.proto.lidar.v1.InfoResponse,
  /**
   * @param {!proto.proto.lidar.v1.InfoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.InfoResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.lidar.v1.InfoRequest,
 *   !proto.proto.lidar.v1.InfoResponse>}
 */
const methodInfo_LidarService_Info = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.lidar.v1.InfoResponse,
  /**
   * @param {!proto.proto.lidar.v1.InfoRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.InfoResponse.deserializeBinary
);


/**
 * @param {!proto.proto.lidar.v1.InfoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.lidar.v1.InfoResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.lidar.v1.InfoResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.lidar.v1.LidarServiceClient.prototype.info =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Info',
      request,
      metadata || {},
      methodDescriptor_LidarService_Info,
      callback);
};


/**
 * @param {!proto.proto.lidar.v1.InfoRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.lidar.v1.InfoResponse>}
 *     Promise that resolves to the response
 */
proto.proto.lidar.v1.LidarServicePromiseClient.prototype.info =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Info',
      request,
      metadata || {},
      methodDescriptor_LidarService_Info);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.lidar.v1.StartRequest,
 *   !proto.proto.lidar.v1.StartResponse>}
 */
const methodDescriptor_LidarService_Start = new grpc.web.MethodDescriptor(
  '/proto.lidar.v1.LidarService/Start',
  grpc.web.MethodType.UNARY,
  proto.proto.lidar.v1.StartRequest,
  proto.proto.lidar.v1.StartResponse,
  /**
   * @param {!proto.proto.lidar.v1.StartRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.StartResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.lidar.v1.StartRequest,
 *   !proto.proto.lidar.v1.StartResponse>}
 */
const methodInfo_LidarService_Start = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.lidar.v1.StartResponse,
  /**
   * @param {!proto.proto.lidar.v1.StartRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.StartResponse.deserializeBinary
);


/**
 * @param {!proto.proto.lidar.v1.StartRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.lidar.v1.StartResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.lidar.v1.StartResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.lidar.v1.LidarServiceClient.prototype.start =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Start',
      request,
      metadata || {},
      methodDescriptor_LidarService_Start,
      callback);
};


/**
 * @param {!proto.proto.lidar.v1.StartRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.lidar.v1.StartResponse>}
 *     Promise that resolves to the response
 */
proto.proto.lidar.v1.LidarServicePromiseClient.prototype.start =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Start',
      request,
      metadata || {},
      methodDescriptor_LidarService_Start);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.lidar.v1.StopRequest,
 *   !proto.proto.lidar.v1.StopResponse>}
 */
const methodDescriptor_LidarService_Stop = new grpc.web.MethodDescriptor(
  '/proto.lidar.v1.LidarService/Stop',
  grpc.web.MethodType.UNARY,
  proto.proto.lidar.v1.StopRequest,
  proto.proto.lidar.v1.StopResponse,
  /**
   * @param {!proto.proto.lidar.v1.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.StopResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.lidar.v1.StopRequest,
 *   !proto.proto.lidar.v1.StopResponse>}
 */
const methodInfo_LidarService_Stop = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.lidar.v1.StopResponse,
  /**
   * @param {!proto.proto.lidar.v1.StopRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.StopResponse.deserializeBinary
);


/**
 * @param {!proto.proto.lidar.v1.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.lidar.v1.StopResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.lidar.v1.StopResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.lidar.v1.LidarServiceClient.prototype.stop =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Stop',
      request,
      metadata || {},
      methodDescriptor_LidarService_Stop,
      callback);
};


/**
 * @param {!proto.proto.lidar.v1.StopRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.lidar.v1.StopResponse>}
 *     Promise that resolves to the response
 */
proto.proto.lidar.v1.LidarServicePromiseClient.prototype.stop =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Stop',
      request,
      metadata || {},
      methodDescriptor_LidarService_Stop);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.lidar.v1.ScanRequest,
 *   !proto.proto.lidar.v1.ScanResponse>}
 */
const methodDescriptor_LidarService_Scan = new grpc.web.MethodDescriptor(
  '/proto.lidar.v1.LidarService/Scan',
  grpc.web.MethodType.UNARY,
  proto.proto.lidar.v1.ScanRequest,
  proto.proto.lidar.v1.ScanResponse,
  /**
   * @param {!proto.proto.lidar.v1.ScanRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.ScanResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.lidar.v1.ScanRequest,
 *   !proto.proto.lidar.v1.ScanResponse>}
 */
const methodInfo_LidarService_Scan = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.lidar.v1.ScanResponse,
  /**
   * @param {!proto.proto.lidar.v1.ScanRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.ScanResponse.deserializeBinary
);


/**
 * @param {!proto.proto.lidar.v1.ScanRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.lidar.v1.ScanResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.lidar.v1.ScanResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.lidar.v1.LidarServiceClient.prototype.scan =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Scan',
      request,
      metadata || {},
      methodDescriptor_LidarService_Scan,
      callback);
};


/**
 * @param {!proto.proto.lidar.v1.ScanRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.lidar.v1.ScanResponse>}
 *     Promise that resolves to the response
 */
proto.proto.lidar.v1.LidarServicePromiseClient.prototype.scan =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Scan',
      request,
      metadata || {},
      methodDescriptor_LidarService_Scan);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.lidar.v1.RangeRequest,
 *   !proto.proto.lidar.v1.RangeResponse>}
 */
const methodDescriptor_LidarService_Range = new grpc.web.MethodDescriptor(
  '/proto.lidar.v1.LidarService/Range',
  grpc.web.MethodType.UNARY,
  proto.proto.lidar.v1.RangeRequest,
  proto.proto.lidar.v1.RangeResponse,
  /**
   * @param {!proto.proto.lidar.v1.RangeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.RangeResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.lidar.v1.RangeRequest,
 *   !proto.proto.lidar.v1.RangeResponse>}
 */
const methodInfo_LidarService_Range = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.lidar.v1.RangeResponse,
  /**
   * @param {!proto.proto.lidar.v1.RangeRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.RangeResponse.deserializeBinary
);


/**
 * @param {!proto.proto.lidar.v1.RangeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.lidar.v1.RangeResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.lidar.v1.RangeResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.lidar.v1.LidarServiceClient.prototype.range =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Range',
      request,
      metadata || {},
      methodDescriptor_LidarService_Range,
      callback);
};


/**
 * @param {!proto.proto.lidar.v1.RangeRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.lidar.v1.RangeResponse>}
 *     Promise that resolves to the response
 */
proto.proto.lidar.v1.LidarServicePromiseClient.prototype.range =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Range',
      request,
      metadata || {},
      methodDescriptor_LidarService_Range);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.lidar.v1.BoundsRequest,
 *   !proto.proto.lidar.v1.BoundsResponse>}
 */
const methodDescriptor_LidarService_Bounds = new grpc.web.MethodDescriptor(
  '/proto.lidar.v1.LidarService/Bounds',
  grpc.web.MethodType.UNARY,
  proto.proto.lidar.v1.BoundsRequest,
  proto.proto.lidar.v1.BoundsResponse,
  /**
   * @param {!proto.proto.lidar.v1.BoundsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.BoundsResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.lidar.v1.BoundsRequest,
 *   !proto.proto.lidar.v1.BoundsResponse>}
 */
const methodInfo_LidarService_Bounds = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.lidar.v1.BoundsResponse,
  /**
   * @param {!proto.proto.lidar.v1.BoundsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.BoundsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.lidar.v1.BoundsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.lidar.v1.BoundsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.lidar.v1.BoundsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.lidar.v1.LidarServiceClient.prototype.bounds =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Bounds',
      request,
      metadata || {},
      methodDescriptor_LidarService_Bounds,
      callback);
};


/**
 * @param {!proto.proto.lidar.v1.BoundsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.lidar.v1.BoundsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.lidar.v1.LidarServicePromiseClient.prototype.bounds =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/Bounds',
      request,
      metadata || {},
      methodDescriptor_LidarService_Bounds);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.lidar.v1.AngularResolutionRequest,
 *   !proto.proto.lidar.v1.AngularResolutionResponse>}
 */
const methodDescriptor_LidarService_AngularResolution = new grpc.web.MethodDescriptor(
  '/proto.lidar.v1.LidarService/AngularResolution',
  grpc.web.MethodType.UNARY,
  proto.proto.lidar.v1.AngularResolutionRequest,
  proto.proto.lidar.v1.AngularResolutionResponse,
  /**
   * @param {!proto.proto.lidar.v1.AngularResolutionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.AngularResolutionResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.lidar.v1.AngularResolutionRequest,
 *   !proto.proto.lidar.v1.AngularResolutionResponse>}
 */
const methodInfo_LidarService_AngularResolution = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.lidar.v1.AngularResolutionResponse,
  /**
   * @param {!proto.proto.lidar.v1.AngularResolutionRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.lidar.v1.AngularResolutionResponse.deserializeBinary
);


/**
 * @param {!proto.proto.lidar.v1.AngularResolutionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.lidar.v1.AngularResolutionResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.lidar.v1.AngularResolutionResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.lidar.v1.LidarServiceClient.prototype.angularResolution =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/AngularResolution',
      request,
      metadata || {},
      methodDescriptor_LidarService_AngularResolution,
      callback);
};


/**
 * @param {!proto.proto.lidar.v1.AngularResolutionRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.lidar.v1.AngularResolutionResponse>}
 *     Promise that resolves to the response
 */
proto.proto.lidar.v1.LidarServicePromiseClient.prototype.angularResolution =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.lidar.v1.LidarService/AngularResolution',
      request,
      metadata || {},
      methodDescriptor_LidarService_AngularResolution);
};


module.exports = proto.proto.lidar.v1;

