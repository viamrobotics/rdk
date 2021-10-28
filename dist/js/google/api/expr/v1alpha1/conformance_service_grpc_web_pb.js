/**
 * @fileoverview gRPC-Web generated client stub for google.api.expr.v1alpha1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_expr_v1alpha1_checked_pb = require('../../../../google/api/expr/v1alpha1/checked_pb.js')

var google_api_expr_v1alpha1_eval_pb = require('../../../../google/api/expr/v1alpha1/eval_pb.js')

var google_api_expr_v1alpha1_syntax_pb = require('../../../../google/api/expr/v1alpha1/syntax_pb.js')

var google_rpc_status_pb = require('../../../../google/rpc/status_pb.js')
const proto = {};
proto.google = {};
proto.google.api = {};
proto.google.api.expr = {};
proto.google.api.expr.v1alpha1 = require('./conformance_service_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.google.api.expr.v1alpha1.ConformanceServiceClient =
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
proto.google.api.expr.v1alpha1.ConformanceServicePromiseClient =
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
 *   !proto.google.api.expr.v1alpha1.ParseRequest,
 *   !proto.google.api.expr.v1alpha1.ParseResponse>}
 */
const methodDescriptor_ConformanceService_Parse = new grpc.web.MethodDescriptor(
  '/google.api.expr.v1alpha1.ConformanceService/Parse',
  grpc.web.MethodType.UNARY,
  proto.google.api.expr.v1alpha1.ParseRequest,
  proto.google.api.expr.v1alpha1.ParseResponse,
  /**
   * @param {!proto.google.api.expr.v1alpha1.ParseRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.expr.v1alpha1.ParseResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.google.api.expr.v1alpha1.ParseRequest,
 *   !proto.google.api.expr.v1alpha1.ParseResponse>}
 */
const methodInfo_ConformanceService_Parse = new grpc.web.AbstractClientBase.MethodInfo(
  proto.google.api.expr.v1alpha1.ParseResponse,
  /**
   * @param {!proto.google.api.expr.v1alpha1.ParseRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.expr.v1alpha1.ParseResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.expr.v1alpha1.ParseRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.google.api.expr.v1alpha1.ParseResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.expr.v1alpha1.ParseResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.expr.v1alpha1.ConformanceServiceClient.prototype.parse =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.expr.v1alpha1.ConformanceService/Parse',
      request,
      metadata || {},
      methodDescriptor_ConformanceService_Parse,
      callback);
};


/**
 * @param {!proto.google.api.expr.v1alpha1.ParseRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.expr.v1alpha1.ParseResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.expr.v1alpha1.ConformanceServicePromiseClient.prototype.parse =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.expr.v1alpha1.ConformanceService/Parse',
      request,
      metadata || {},
      methodDescriptor_ConformanceService_Parse);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.expr.v1alpha1.CheckRequest,
 *   !proto.google.api.expr.v1alpha1.CheckResponse>}
 */
const methodDescriptor_ConformanceService_Check = new grpc.web.MethodDescriptor(
  '/google.api.expr.v1alpha1.ConformanceService/Check',
  grpc.web.MethodType.UNARY,
  proto.google.api.expr.v1alpha1.CheckRequest,
  proto.google.api.expr.v1alpha1.CheckResponse,
  /**
   * @param {!proto.google.api.expr.v1alpha1.CheckRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.expr.v1alpha1.CheckResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.google.api.expr.v1alpha1.CheckRequest,
 *   !proto.google.api.expr.v1alpha1.CheckResponse>}
 */
const methodInfo_ConformanceService_Check = new grpc.web.AbstractClientBase.MethodInfo(
  proto.google.api.expr.v1alpha1.CheckResponse,
  /**
   * @param {!proto.google.api.expr.v1alpha1.CheckRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.expr.v1alpha1.CheckResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.expr.v1alpha1.CheckRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.google.api.expr.v1alpha1.CheckResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.expr.v1alpha1.CheckResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.expr.v1alpha1.ConformanceServiceClient.prototype.check =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.expr.v1alpha1.ConformanceService/Check',
      request,
      metadata || {},
      methodDescriptor_ConformanceService_Check,
      callback);
};


/**
 * @param {!proto.google.api.expr.v1alpha1.CheckRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.expr.v1alpha1.CheckResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.expr.v1alpha1.ConformanceServicePromiseClient.prototype.check =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.expr.v1alpha1.ConformanceService/Check',
      request,
      metadata || {},
      methodDescriptor_ConformanceService_Check);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.expr.v1alpha1.EvalRequest,
 *   !proto.google.api.expr.v1alpha1.EvalResponse>}
 */
const methodDescriptor_ConformanceService_Eval = new grpc.web.MethodDescriptor(
  '/google.api.expr.v1alpha1.ConformanceService/Eval',
  grpc.web.MethodType.UNARY,
  proto.google.api.expr.v1alpha1.EvalRequest,
  proto.google.api.expr.v1alpha1.EvalResponse,
  /**
   * @param {!proto.google.api.expr.v1alpha1.EvalRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.expr.v1alpha1.EvalResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.google.api.expr.v1alpha1.EvalRequest,
 *   !proto.google.api.expr.v1alpha1.EvalResponse>}
 */
const methodInfo_ConformanceService_Eval = new grpc.web.AbstractClientBase.MethodInfo(
  proto.google.api.expr.v1alpha1.EvalResponse,
  /**
   * @param {!proto.google.api.expr.v1alpha1.EvalRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.expr.v1alpha1.EvalResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.expr.v1alpha1.EvalRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.google.api.expr.v1alpha1.EvalResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.expr.v1alpha1.EvalResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.expr.v1alpha1.ConformanceServiceClient.prototype.eval =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.expr.v1alpha1.ConformanceService/Eval',
      request,
      metadata || {},
      methodDescriptor_ConformanceService_Eval,
      callback);
};


/**
 * @param {!proto.google.api.expr.v1alpha1.EvalRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.expr.v1alpha1.EvalResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.expr.v1alpha1.ConformanceServicePromiseClient.prototype.eval =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.expr.v1alpha1.ConformanceService/Eval',
      request,
      metadata || {},
      methodDescriptor_ConformanceService_Eval);
};


module.exports = proto.google.api.expr.v1alpha1;

