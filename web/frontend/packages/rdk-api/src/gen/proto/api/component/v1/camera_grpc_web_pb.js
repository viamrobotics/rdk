/**
 * @fileoverview gRPC-Web generated client stub for proto.api.component.v1
 * @enhanceable
 * @public
 */

// GENERATED CODE -- DO NOT EDIT!


/* eslint-disable */
// @ts-nocheck



const grpc = {};
grpc.web = require('grpc-web');


var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var google_api_httpbody_pb = require('../../../../google/api/httpbody_pb.js')

var proto_api_common_v1_common_pb = require('../../../../proto/api/common/v1/common_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.component = {};
proto.proto.api.component.v1 = require('./camera_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.CameraServiceClient =
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
proto.proto.api.component.v1.CameraServicePromiseClient =
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
 *   !proto.proto.api.component.v1.CameraServiceGetFrameRequest,
 *   !proto.proto.api.component.v1.CameraServiceGetFrameResponse>}
 */
const methodDescriptor_CameraService_GetFrame = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.CameraService/GetFrame',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.CameraServiceGetFrameRequest,
  proto.proto.api.component.v1.CameraServiceGetFrameResponse,
  /**
   * @param {!proto.proto.api.component.v1.CameraServiceGetFrameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.CameraServiceGetFrameResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.CameraServiceGetFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.CameraServiceGetFrameResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.CameraServiceGetFrameResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.CameraServiceClient.prototype.getFrame =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/GetFrame',
      request,
      metadata || {},
      methodDescriptor_CameraService_GetFrame,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.CameraServiceGetFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.CameraServiceGetFrameResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.CameraServicePromiseClient.prototype.getFrame =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/GetFrame',
      request,
      metadata || {},
      methodDescriptor_CameraService_GetFrame);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.CameraServiceRenderFrameRequest,
 *   !proto.google.api.HttpBody>}
 */
const methodDescriptor_CameraService_RenderFrame = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.CameraService/RenderFrame',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.CameraServiceRenderFrameRequest,
  google_api_httpbody_pb.HttpBody,
  /**
   * @param {!proto.proto.api.component.v1.CameraServiceRenderFrameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  google_api_httpbody_pb.HttpBody.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.CameraServiceRenderFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.google.api.HttpBody)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.google.api.HttpBody>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.CameraServiceClient.prototype.renderFrame =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/RenderFrame',
      request,
      metadata || {},
      methodDescriptor_CameraService_RenderFrame,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.CameraServiceRenderFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.google.api.HttpBody>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.CameraServicePromiseClient.prototype.renderFrame =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/RenderFrame',
      request,
      metadata || {},
      methodDescriptor_CameraService_RenderFrame);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.CameraServiceGetPointCloudRequest,
 *   !proto.proto.api.component.v1.CameraServiceGetPointCloudResponse>}
 */
const methodDescriptor_CameraService_GetPointCloud = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.CameraService/GetPointCloud',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.CameraServiceGetPointCloudRequest,
  proto.proto.api.component.v1.CameraServiceGetPointCloudResponse,
  /**
   * @param {!proto.proto.api.component.v1.CameraServiceGetPointCloudRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.CameraServiceGetPointCloudResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.CameraServiceGetPointCloudRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.CameraServiceGetPointCloudResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.CameraServiceGetPointCloudResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.CameraServiceClient.prototype.getPointCloud =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/GetPointCloud',
      request,
      metadata || {},
      methodDescriptor_CameraService_GetPointCloud,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.CameraServiceGetPointCloudRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.CameraServiceGetPointCloudResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.CameraServicePromiseClient.prototype.getPointCloud =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/GetPointCloud',
      request,
      metadata || {},
      methodDescriptor_CameraService_GetPointCloud);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsRequest,
 *   !proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsResponse>}
 */
const methodDescriptor_CameraService_GetObjectPointClouds = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.CameraService/GetObjectPointClouds',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsRequest,
  proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsResponse,
  /**
   * @param {!proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.CameraServiceClient.prototype.getObjectPointClouds =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/GetObjectPointClouds',
      request,
      metadata || {},
      methodDescriptor_CameraService_GetObjectPointClouds,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.CameraServiceGetObjectPointCloudsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.CameraServicePromiseClient.prototype.getObjectPointClouds =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/GetObjectPointClouds',
      request,
      metadata || {},
      methodDescriptor_CameraService_GetObjectPointClouds);
};


module.exports = proto.proto.api.component.v1;

