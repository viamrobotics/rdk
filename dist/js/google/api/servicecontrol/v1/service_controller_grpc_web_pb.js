/**
 * @fileoverview gRPC-Web generated client stub for google.api.servicecontrol.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var google_api_servicecontrol_v1_check_error_pb = require('../../../../google/api/servicecontrol/v1/check_error_pb.js')

var google_api_servicecontrol_v1_operation_pb = require('../../../../google/api/servicecontrol/v1/operation_pb.js')

var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js')

var google_rpc_status_pb = require('../../../../google/rpc/status_pb.js')

var google_api_client_pb = require('../../../../google/api/client_pb.js')
const proto = {};
proto.google = {};
proto.google.api = {};
proto.google.api.servicecontrol = {};
proto.google.api.servicecontrol.v1 = require('./service_controller_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.google.api.servicecontrol.v1.ServiceControllerClient =
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
proto.google.api.servicecontrol.v1.ServiceControllerPromiseClient =
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
 *   !proto.google.api.servicecontrol.v1.CheckRequest,
 *   !proto.google.api.servicecontrol.v1.CheckResponse>}
 */
const methodDescriptor_ServiceController_Check = new grpc.web.MethodDescriptor(
  '/google.api.servicecontrol.v1.ServiceController/Check',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicecontrol.v1.CheckRequest,
  proto.google.api.servicecontrol.v1.CheckResponse,
  /**
   * @param {!proto.google.api.servicecontrol.v1.CheckRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicecontrol.v1.CheckResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.google.api.servicecontrol.v1.CheckRequest,
 *   !proto.google.api.servicecontrol.v1.CheckResponse>}
 */
const methodInfo_ServiceController_Check = new grpc.web.AbstractClientBase.MethodInfo(
  proto.google.api.servicecontrol.v1.CheckResponse,
  /**
   * @param {!proto.google.api.servicecontrol.v1.CheckRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicecontrol.v1.CheckResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.servicecontrol.v1.CheckRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.google.api.servicecontrol.v1.CheckResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicecontrol.v1.CheckResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicecontrol.v1.ServiceControllerClient.prototype.check =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicecontrol.v1.ServiceController/Check',
      request,
      metadata || {},
      methodDescriptor_ServiceController_Check,
      callback);
};


/**
 * @param {!proto.google.api.servicecontrol.v1.CheckRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicecontrol.v1.CheckResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.servicecontrol.v1.ServiceControllerPromiseClient.prototype.check =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicecontrol.v1.ServiceController/Check',
      request,
      metadata || {},
      methodDescriptor_ServiceController_Check);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicecontrol.v1.ReportRequest,
 *   !proto.google.api.servicecontrol.v1.ReportResponse>}
 */
const methodDescriptor_ServiceController_Report = new grpc.web.MethodDescriptor(
  '/google.api.servicecontrol.v1.ServiceController/Report',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicecontrol.v1.ReportRequest,
  proto.google.api.servicecontrol.v1.ReportResponse,
  /**
   * @param {!proto.google.api.servicecontrol.v1.ReportRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicecontrol.v1.ReportResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.google.api.servicecontrol.v1.ReportRequest,
 *   !proto.google.api.servicecontrol.v1.ReportResponse>}
 */
const methodInfo_ServiceController_Report = new grpc.web.AbstractClientBase.MethodInfo(
  proto.google.api.servicecontrol.v1.ReportResponse,
  /**
   * @param {!proto.google.api.servicecontrol.v1.ReportRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicecontrol.v1.ReportResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.servicecontrol.v1.ReportRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.google.api.servicecontrol.v1.ReportResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicecontrol.v1.ReportResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicecontrol.v1.ServiceControllerClient.prototype.report =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicecontrol.v1.ServiceController/Report',
      request,
      metadata || {},
      methodDescriptor_ServiceController_Report,
      callback);
};


/**
 * @param {!proto.google.api.servicecontrol.v1.ReportRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicecontrol.v1.ReportResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.servicecontrol.v1.ServiceControllerPromiseClient.prototype.report =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicecontrol.v1.ServiceController/Report',
      request,
      metadata || {},
      methodDescriptor_ServiceController_Report);
};


module.exports = proto.google.api.servicecontrol.v1;

