/**
 * @fileoverview gRPC-Web generated client stub for google.api.serviceusage.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var google_api_serviceusage_v1_resources_pb = require('../../../../google/api/serviceusage/v1/resources_pb.js')

var google_longrunning_operations_pb = require('../../../../google/longrunning/operations_pb.js')

var google_api_client_pb = require('../../../../google/api/client_pb.js')
const proto = {};
proto.google = {};
proto.google.api = {};
proto.google.api.serviceusage = {};
proto.google.api.serviceusage.v1 = require('./serviceusage_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.google.api.serviceusage.v1.ServiceUsageClient =
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
proto.google.api.serviceusage.v1.ServiceUsagePromiseClient =
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
 *   !proto.google.api.serviceusage.v1.EnableServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_EnableService = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1.ServiceUsage/EnableService',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1.EnableServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1.EnableServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1.EnableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1.ServiceUsageClient.prototype.enableService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/EnableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_EnableService,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1.EnableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1.ServiceUsagePromiseClient.prototype.enableService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/EnableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_EnableService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1.DisableServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_DisableService = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1.ServiceUsage/DisableService',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1.DisableServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1.DisableServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1.DisableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1.ServiceUsageClient.prototype.disableService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/DisableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DisableService,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1.DisableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1.ServiceUsagePromiseClient.prototype.disableService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/DisableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DisableService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1.GetServiceRequest,
 *   !proto.google.api.serviceusage.v1.Service>}
 */
const methodDescriptor_ServiceUsage_GetService = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1.ServiceUsage/GetService',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1.GetServiceRequest,
  google_api_serviceusage_v1_resources_pb.Service,
  /**
   * @param {!proto.google.api.serviceusage.v1.GetServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_serviceusage_v1_resources_pb.Service.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1.GetServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1.Service)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1.Service>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1.ServiceUsageClient.prototype.getService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/GetService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetService,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1.GetServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1.Service>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1.ServiceUsagePromiseClient.prototype.getService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/GetService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1.ListServicesRequest,
 *   !proto.google.api.serviceusage.v1.ListServicesResponse>}
 */
const methodDescriptor_ServiceUsage_ListServices = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1.ServiceUsage/ListServices',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1.ListServicesRequest,
  proto.google.api.serviceusage.v1.ListServicesResponse,
  /**
   * @param {!proto.google.api.serviceusage.v1.ListServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.serviceusage.v1.ListServicesResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1.ListServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1.ListServicesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1.ListServicesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1.ServiceUsageClient.prototype.listServices =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/ListServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListServices,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1.ListServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1.ListServicesResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1.ServiceUsagePromiseClient.prototype.listServices =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/ListServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListServices);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1.BatchEnableServicesRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_BatchEnableServices = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1.ServiceUsage/BatchEnableServices',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1.BatchEnableServicesRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1.BatchEnableServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1.BatchEnableServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1.ServiceUsageClient.prototype.batchEnableServices =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/BatchEnableServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_BatchEnableServices,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1.BatchEnableServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1.ServiceUsagePromiseClient.prototype.batchEnableServices =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/BatchEnableServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_BatchEnableServices);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1.BatchGetServicesRequest,
 *   !proto.google.api.serviceusage.v1.BatchGetServicesResponse>}
 */
const methodDescriptor_ServiceUsage_BatchGetServices = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1.ServiceUsage/BatchGetServices',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1.BatchGetServicesRequest,
  proto.google.api.serviceusage.v1.BatchGetServicesResponse,
  /**
   * @param {!proto.google.api.serviceusage.v1.BatchGetServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.serviceusage.v1.BatchGetServicesResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1.BatchGetServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1.BatchGetServicesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1.BatchGetServicesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1.ServiceUsageClient.prototype.batchGetServices =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/BatchGetServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_BatchGetServices,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1.BatchGetServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1.BatchGetServicesResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1.ServiceUsagePromiseClient.prototype.batchGetServices =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1.ServiceUsage/BatchGetServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_BatchGetServices);
};


module.exports = proto.google.api.serviceusage.v1;

