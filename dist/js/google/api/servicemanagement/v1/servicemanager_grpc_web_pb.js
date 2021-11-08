/**
 * @fileoverview gRPC-Web generated client stub for google.api.servicemanagement.v1
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

var google_api_service_pb = require('../../../../google/api/service_pb.js')

var google_api_servicemanagement_v1_resources_pb = require('../../../../google/api/servicemanagement/v1/resources_pb.js')

var google_longrunning_operations_pb = require('../../../../google/longrunning/operations_pb.js')

var google_protobuf_any_pb = require('google-protobuf/google/protobuf/any_pb.js')

var google_protobuf_field_mask_pb = require('google-protobuf/google/protobuf/field_mask_pb.js')

var google_rpc_status_pb = require('../../../../google/rpc/status_pb.js')
const proto = {};
proto.google = {};
proto.google.api = {};
proto.google.api.servicemanagement = {};
proto.google.api.servicemanagement.v1 = require('./servicemanager_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient =
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
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient =
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
 *   !proto.google.api.servicemanagement.v1.ListServicesRequest,
 *   !proto.google.api.servicemanagement.v1.ListServicesResponse>}
 */
const methodDescriptor_ServiceManager_ListServices = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/ListServices',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.ListServicesRequest,
  proto.google.api.servicemanagement.v1.ListServicesResponse,
  /**
   * @param {!proto.google.api.servicemanagement.v1.ListServicesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicemanagement.v1.ListServicesResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.ListServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.servicemanagement.v1.ListServicesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicemanagement.v1.ListServicesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.listServices =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/ListServices',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_ListServices,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.ListServicesRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicemanagement.v1.ListServicesResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.listServices =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/ListServices',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_ListServices);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.GetServiceRequest,
 *   !proto.google.api.servicemanagement.v1.ManagedService>}
 */
const methodDescriptor_ServiceManager_GetService = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/GetService',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.GetServiceRequest,
  google_api_servicemanagement_v1_resources_pb.ManagedService,
  /**
   * @param {!proto.google.api.servicemanagement.v1.GetServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_servicemanagement_v1_resources_pb.ManagedService.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.GetServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.servicemanagement.v1.ManagedService)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicemanagement.v1.ManagedService>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.getService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GetService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GetService,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.GetServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicemanagement.v1.ManagedService>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.getService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GetService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GetService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.CreateServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceManager_CreateService = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/CreateService',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.CreateServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.servicemanagement.v1.CreateServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.CreateServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.createService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/CreateService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_CreateService,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.CreateServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.createService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/CreateService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_CreateService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.DeleteServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceManager_DeleteService = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/DeleteService',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.DeleteServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.servicemanagement.v1.DeleteServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.DeleteServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.deleteService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/DeleteService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_DeleteService,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.DeleteServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.deleteService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/DeleteService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_DeleteService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.UndeleteServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceManager_UndeleteService = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/UndeleteService',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.UndeleteServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.servicemanagement.v1.UndeleteServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.UndeleteServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.undeleteService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/UndeleteService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_UndeleteService,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.UndeleteServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.undeleteService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/UndeleteService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_UndeleteService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.ListServiceConfigsRequest,
 *   !proto.google.api.servicemanagement.v1.ListServiceConfigsResponse>}
 */
const methodDescriptor_ServiceManager_ListServiceConfigs = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/ListServiceConfigs',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.ListServiceConfigsRequest,
  proto.google.api.servicemanagement.v1.ListServiceConfigsResponse,
  /**
   * @param {!proto.google.api.servicemanagement.v1.ListServiceConfigsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicemanagement.v1.ListServiceConfigsResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.ListServiceConfigsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.servicemanagement.v1.ListServiceConfigsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicemanagement.v1.ListServiceConfigsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.listServiceConfigs =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/ListServiceConfigs',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_ListServiceConfigs,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.ListServiceConfigsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicemanagement.v1.ListServiceConfigsResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.listServiceConfigs =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/ListServiceConfigs',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_ListServiceConfigs);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.GetServiceConfigRequest,
 *   !proto.google.api.Service>}
 */
const methodDescriptor_ServiceManager_GetServiceConfig = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/GetServiceConfig',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.GetServiceConfigRequest,
  google_api_service_pb.Service,
  /**
   * @param {!proto.google.api.servicemanagement.v1.GetServiceConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_service_pb.Service.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.GetServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.Service)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.Service>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.getServiceConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GetServiceConfig',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GetServiceConfig,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.GetServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.Service>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.getServiceConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GetServiceConfig',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GetServiceConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.CreateServiceConfigRequest,
 *   !proto.google.api.Service>}
 */
const methodDescriptor_ServiceManager_CreateServiceConfig = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/CreateServiceConfig',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.CreateServiceConfigRequest,
  google_api_service_pb.Service,
  /**
   * @param {!proto.google.api.servicemanagement.v1.CreateServiceConfigRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_service_pb.Service.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.CreateServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.Service)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.Service>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.createServiceConfig =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/CreateServiceConfig',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_CreateServiceConfig,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.CreateServiceConfigRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.Service>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.createServiceConfig =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/CreateServiceConfig',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_CreateServiceConfig);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.SubmitConfigSourceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceManager_SubmitConfigSource = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/SubmitConfigSource',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.SubmitConfigSourceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.servicemanagement.v1.SubmitConfigSourceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.SubmitConfigSourceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.submitConfigSource =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/SubmitConfigSource',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_SubmitConfigSource,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.SubmitConfigSourceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.submitConfigSource =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/SubmitConfigSource',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_SubmitConfigSource);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.ListServiceRolloutsRequest,
 *   !proto.google.api.servicemanagement.v1.ListServiceRolloutsResponse>}
 */
const methodDescriptor_ServiceManager_ListServiceRollouts = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/ListServiceRollouts',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.ListServiceRolloutsRequest,
  proto.google.api.servicemanagement.v1.ListServiceRolloutsResponse,
  /**
   * @param {!proto.google.api.servicemanagement.v1.ListServiceRolloutsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicemanagement.v1.ListServiceRolloutsResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.ListServiceRolloutsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.servicemanagement.v1.ListServiceRolloutsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicemanagement.v1.ListServiceRolloutsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.listServiceRollouts =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/ListServiceRollouts',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_ListServiceRollouts,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.ListServiceRolloutsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicemanagement.v1.ListServiceRolloutsResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.listServiceRollouts =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/ListServiceRollouts',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_ListServiceRollouts);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.GetServiceRolloutRequest,
 *   !proto.google.api.servicemanagement.v1.Rollout>}
 */
const methodDescriptor_ServiceManager_GetServiceRollout = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/GetServiceRollout',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.GetServiceRolloutRequest,
  google_api_servicemanagement_v1_resources_pb.Rollout,
  /**
   * @param {!proto.google.api.servicemanagement.v1.GetServiceRolloutRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_servicemanagement_v1_resources_pb.Rollout.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.GetServiceRolloutRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.servicemanagement.v1.Rollout)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicemanagement.v1.Rollout>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.getServiceRollout =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GetServiceRollout',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GetServiceRollout,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.GetServiceRolloutRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicemanagement.v1.Rollout>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.getServiceRollout =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GetServiceRollout',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GetServiceRollout);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.CreateServiceRolloutRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceManager_CreateServiceRollout = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/CreateServiceRollout',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.CreateServiceRolloutRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.servicemanagement.v1.CreateServiceRolloutRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.CreateServiceRolloutRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.createServiceRollout =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/CreateServiceRollout',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_CreateServiceRollout,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.CreateServiceRolloutRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.createServiceRollout =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/CreateServiceRollout',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_CreateServiceRollout);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.GenerateConfigReportRequest,
 *   !proto.google.api.servicemanagement.v1.GenerateConfigReportResponse>}
 */
const methodDescriptor_ServiceManager_GenerateConfigReport = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/GenerateConfigReport',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.GenerateConfigReportRequest,
  proto.google.api.servicemanagement.v1.GenerateConfigReportResponse,
  /**
   * @param {!proto.google.api.servicemanagement.v1.GenerateConfigReportRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.google.api.servicemanagement.v1.GenerateConfigReportResponse.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.GenerateConfigReportRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.servicemanagement.v1.GenerateConfigReportResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.servicemanagement.v1.GenerateConfigReportResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.generateConfigReport =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GenerateConfigReport',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GenerateConfigReport,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.GenerateConfigReportRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.servicemanagement.v1.GenerateConfigReportResponse>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.generateConfigReport =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/GenerateConfigReport',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_GenerateConfigReport);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.EnableServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceManager_EnableService = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/EnableService',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.EnableServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.servicemanagement.v1.EnableServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.EnableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.enableService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/EnableService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_EnableService,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.EnableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.enableService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/EnableService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_EnableService);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.google.api.servicemanagement.v1.DisableServiceRequest,
 *   !proto.google.longrunning.Operation>}
 */
const methodDescriptor_ServiceManager_DisableService = new grpc.web.MethodDescriptor(
  '/google.api.servicemanagement.v1.ServiceManager/DisableService',
  grpc.web.MethodType.UNARY,
  proto.google.api.servicemanagement.v1.DisableServiceRequest,
  google_longrunning_operations_pb.Operation,
  /**
   * @param {!proto.google.api.servicemanagement.v1.DisableServiceRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_longrunning_operations_pb.Operation.deserializeBinary
);


/**
 * @param {!proto.google.api.servicemanagement.v1.DisableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.longrunning.Operation)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.longrunning.Operation>|undefined}
 *     The XHR Node Readable Stream
 */
proto.google.api.servicemanagement.v1.ServiceManagerClient.prototype.disableService =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/DisableService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_DisableService,
      callback);
};


/**
 * @param {!proto.google.api.servicemanagement.v1.DisableServiceRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.longrunning.Operation>}
 *     Promise that resolves to the response
 */
proto.google.api.servicemanagement.v1.ServiceManagerPromiseClient.prototype.disableService =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/google.api.servicemanagement.v1.ServiceManager/DisableService',
      request,
      metadata || {},
      methodDescriptor_ServiceManager_DisableService);
};


module.exports = proto.google.api.servicemanagement.v1;

