/**
 * @fileoverview gRPC-Web generated client stub for google.api.serviceusage.v1beta1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var google_api_client_pb = require('../../../../google/api/client_pb.js')

var google_api_field_behavior_pb = require('../../../../google/api/field_behavior_pb.js')

var google_api_serviceusage_v1beta1_resources_pb = require('../../../../google/api/serviceusage/v1beta1/resources_pb.js')

var google_longrunning_operations_pb = require('../../../../google/longrunning/operations_pb.js')

var google_protobuf_field_mask_pb = require('google-protobuf/google/protobuf/field_mask_pb.js')
const proto = {};
proto.google = {};
proto.google.api = {};
proto.google.api.serviceusage = {};
proto.google.api.serviceusage.v1beta1 = require('./serviceusage_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient =
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
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient =
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
 *   !proto.google.api.serviceusage.v1beta1.EnableServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_EnableService = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/EnableService',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.EnableServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.enableService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/EnableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_EnableService,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.enableService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/EnableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_EnableService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.DisableServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_DisableService = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/DisableService',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.DisableServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.disableService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/DisableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DisableService,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.disableService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/DisableService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DisableService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.GetServiceRequest,
 *   !proto.google.api.serviceusage.v1beta1.Service>}
 */
const methodDescriptor_ServiceUsage_GetService = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/GetService',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.GetServiceRequest,
  google_api_serviceusage_v1beta1_resources_pb.Service,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.GetServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_serviceusage_v1beta1_resources_pb.Service.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1beta1.Service)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1beta1.Service>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.getService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GetService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetService,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1beta1.Service>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.getService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GetService',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.ListServicesRequest,
 *   !proto.google.api.serviceusage.v1beta1.ListServicesResponse>}
 */
const methodDescriptor_ServiceUsage_ListServices = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/ListServices',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.ListServicesRequest,
  proto.google.api.serviceusage.v1beta1.ListServicesResponse,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.serviceusage.v1beta1.ListServicesResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1beta1.ListServicesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1beta1.ListServicesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.listServices =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListServices,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1beta1.ListServicesResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.listServices =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListServices);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_BatchEnableServices = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/BatchEnableServices',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.batchEnableServices =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/BatchEnableServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_BatchEnableServices,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.batchEnableServices =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/BatchEnableServices',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_BatchEnableServices);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest,
 *   !proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse>}
 */
const methodDescriptor_ServiceUsage_ListConsumerQuotaMetrics = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/ListConsumerQuotaMetrics',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest,
  proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.listConsumerQuotaMetrics =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListConsumerQuotaMetrics',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListConsumerQuotaMetrics,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.listConsumerQuotaMetrics =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListConsumerQuotaMetrics',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListConsumerQuotaMetrics);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest,
 *   !proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric>}
 */
const methodDescriptor_ServiceUsage_GetConsumerQuotaMetric = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/GetConsumerQuotaMetric',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest,
  google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.getConsumerQuotaMetric =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GetConsumerQuotaMetric',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetConsumerQuotaMetric,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.getConsumerQuotaMetric =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GetConsumerQuotaMetric',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetConsumerQuotaMetric);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest,
 *   !proto.google.api.serviceusage.v1beta1.ConsumerQuotaLimit>}
 */
const methodDescriptor_ServiceUsage_GetConsumerQuotaLimit = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/GetConsumerQuotaLimit',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest,
  google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaLimit,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaLimit.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1beta1.ConsumerQuotaLimit)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1beta1.ConsumerQuotaLimit>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.getConsumerQuotaLimit =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GetConsumerQuotaLimit',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetConsumerQuotaLimit,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1beta1.ConsumerQuotaLimit>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.getConsumerQuotaLimit =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GetConsumerQuotaLimit',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GetConsumerQuotaLimit);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_CreateAdminOverride = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/CreateAdminOverride',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.createAdminOverride =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/CreateAdminOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_CreateAdminOverride,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.createAdminOverride =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/CreateAdminOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_CreateAdminOverride);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_UpdateAdminOverride = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/UpdateAdminOverride',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.updateAdminOverride =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/UpdateAdminOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_UpdateAdminOverride,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.updateAdminOverride =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/UpdateAdminOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_UpdateAdminOverride);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_DeleteAdminOverride = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/DeleteAdminOverride',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.deleteAdminOverride =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/DeleteAdminOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DeleteAdminOverride,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.deleteAdminOverride =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/DeleteAdminOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DeleteAdminOverride);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest,
 *   !proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse>}
 */
const methodDescriptor_ServiceUsage_ListAdminOverrides = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/ListAdminOverrides',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest,
  proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.listAdminOverrides =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListAdminOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListAdminOverrides,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.listAdminOverrides =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListAdminOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListAdminOverrides);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_ImportAdminOverrides = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/ImportAdminOverrides',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.importAdminOverrides =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ImportAdminOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ImportAdminOverrides,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.importAdminOverrides =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ImportAdminOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ImportAdminOverrides);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_CreateConsumerOverride = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/CreateConsumerOverride',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.createConsumerOverride =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/CreateConsumerOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_CreateConsumerOverride,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.createConsumerOverride =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/CreateConsumerOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_CreateConsumerOverride);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_UpdateConsumerOverride = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/UpdateConsumerOverride',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.updateConsumerOverride =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/UpdateConsumerOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_UpdateConsumerOverride,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.updateConsumerOverride =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/UpdateConsumerOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_UpdateConsumerOverride);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_DeleteConsumerOverride = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/DeleteConsumerOverride',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.deleteConsumerOverride =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/DeleteConsumerOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DeleteConsumerOverride,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.deleteConsumerOverride =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/DeleteConsumerOverride',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_DeleteConsumerOverride);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest,
 *   !proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse>}
 */
const methodDescriptor_ServiceUsage_ListConsumerOverrides = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/ListConsumerOverrides',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest,
  proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.listConsumerOverrides =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListConsumerOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListConsumerOverrides,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.listConsumerOverrides =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ListConsumerOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ListConsumerOverrides);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_ImportConsumerOverrides = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/ImportConsumerOverrides',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.importConsumerOverrides =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ImportConsumerOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ImportConsumerOverrides,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.importConsumerOverrides =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/ImportConsumerOverrides',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_ImportConsumerOverrides);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceUsage_GenerateServiceIdentity = new grpc.web.MethodDescriptor(
  '/google.api.serviceusage.v1beta1.ServiceUsage/GenerateServiceIdentity',
  grpc.web.MethodType.UNARY,
  proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.serviceusage.v1beta1.ServiceUsageClient.prototype.generateServiceIdentity =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GenerateServiceIdentity',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GenerateServiceIdentity,
      callback);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.serviceusage.v1beta1.ServiceUsagePromiseClient.prototype.generateServiceIdentity =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.serviceusage.v1beta1.ServiceUsage/GenerateServiceIdentity',
      request,
      metadata || {},
      methodDescriptor_ServiceUsage_GenerateServiceIdentity);
};


module.exports = proto.google.api.serviceusage.v1beta1;

