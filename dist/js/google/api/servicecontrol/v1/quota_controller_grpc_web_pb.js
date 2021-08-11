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

var google_api_servicecontrol_v1_metric_value_pb = require('../../../../google/api/servicecontrol/v1/metric_value_pb.js')

var google_api_client_pb = require('../../../../google/api/client_pb.js')
const proto = {};
proto.google = {};
proto.google.api = {};
proto.google.api.servicecontrol = {};
proto.google.api.servicecontrol.v1 = require('./quota_controller_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.google.api.servicecontrol.v1.QuotaControllerClient =
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
proto.google.api.servicecontrol.v1.QuotaControllerPromiseClient =
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
 *   !proto.google.api.servicecontrol.v1.AllocateQuotaRequest,
 *   !proto.google.api.servicecontrol.v1.AllocateQuotaResponse>}
 */
const methodDescriptor_QuotaController_AllocateQuota = new grpc.web.MethodDescriptor(
  '/google.api.servicecontrol.v1.QuotaController/AllocateQuota',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicecontrol.v1.AllocateQuotaRequest,
  proto.google.api.servicecontrol.v1.AllocateQuotaResponse,
  /**
   * @param {!proto.google.api.servicecontrol.v1.AllocateQuotaRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicecontrol.v1.AllocateQuotaResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.google.api.servicecontrol.v1.AllocateQuotaRequest,
 *   !proto.google.api.servicecontrol.v1.AllocateQuotaResponse>}
 */
const methodInfo_QuotaController_AllocateQuota = new grpc.web.AbstractClientBase.MethodInfo(
  proto.google.api.servicecontrol.v1.AllocateQuotaResponse,
  /**
   * @param {!proto.google.api.servicecontrol.v1.AllocateQuotaRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicecontrol.v1.AllocateQuotaResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.servicecontrol.v1.AllocateQuotaRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.google.api.servicecontrol.v1.AllocateQuotaResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicecontrol.v1.AllocateQuotaResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicecontrol.v1.QuotaControllerClient.prototype.allocateQuota =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicecontrol.v1.QuotaController/AllocateQuota',
      request,
      metadata || {},
      methodDescriptor_QuotaController_AllocateQuota,
      callback);
};


/**
 * @param {!proto.google.api.servicecontrol.v1.AllocateQuotaRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicecontrol.v1.AllocateQuotaResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.servicecontrol.v1.QuotaControllerPromiseClient.prototype.allocateQuota =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicecontrol.v1.QuotaController/AllocateQuota',
      request,
      metadata || {},
      methodDescriptor_QuotaController_AllocateQuota);
};


module.exports = proto.google.api.servicecontrol.v1;

