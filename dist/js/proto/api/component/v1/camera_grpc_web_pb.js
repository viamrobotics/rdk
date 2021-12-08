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
 *   !proto.proto.api.component.v1.CameraServiceFrameRequest,
 *   !proto.proto.api.component.v1.CameraServiceFrameResponse>}
 */
const methodDescriptor_CameraService_Frame = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.CameraService/Frame',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.CameraServiceFrameRequest,
  proto.proto.api.component.v1.CameraServiceFrameResponse,
  /**
   * @param {!proto.proto.api.component.v1.CameraServiceFrameRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.CameraServiceFrameResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.CameraServiceFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.CameraServiceFrameResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.CameraServiceFrameResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.CameraServiceClient.prototype.frame =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/Frame',
      request,
      metadata || {},
      methodDescriptor_CameraService_Frame,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.CameraServiceFrameRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.CameraServiceFrameResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.CameraServicePromiseClient.prototype.frame =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/Frame',
      request,
      metadata || {},
      methodDescriptor_CameraService_Frame);
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
 *   !proto.proto.api.component.v1.CameraServicePointCloudRequest,
 *   !proto.proto.api.component.v1.CameraServicePointCloudResponse>}
 */
const methodDescriptor_CameraService_PointCloud = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.CameraService/PointCloud',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.CameraServicePointCloudRequest,
  proto.proto.api.component.v1.CameraServicePointCloudResponse,
  /**
   * @param {!proto.proto.api.component.v1.CameraServicePointCloudRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.CameraServicePointCloudResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.CameraServicePointCloudRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.CameraServicePointCloudResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.CameraServicePointCloudResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.CameraServiceClient.prototype.pointCloud =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/PointCloud',
      request,
      metadata || {},
      methodDescriptor_CameraService_PointCloud,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.CameraServicePointCloudRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.CameraServicePointCloudResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.CameraServicePromiseClient.prototype.pointCloud =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/PointCloud',
      request,
      metadata || {},
      methodDescriptor_CameraService_PointCloud);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.CameraServiceObjectPointCloudsRequest,
 *   !proto.proto.api.component.v1.CameraServiceObjectPointCloudsResponse>}
 */
const methodDescriptor_CameraService_ObjectPointClouds = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.CameraService/ObjectPointClouds',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.CameraServiceObjectPointCloudsRequest,
  proto.proto.api.component.v1.CameraServiceObjectPointCloudsResponse,
  /**
   * @param {!proto.proto.api.component.v1.CameraServiceObjectPointCloudsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.CameraServiceObjectPointCloudsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.CameraServiceObjectPointCloudsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.CameraServiceObjectPointCloudsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.CameraServiceObjectPointCloudsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.CameraServiceClient.prototype.objectPointClouds =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/ObjectPointClouds',
      request,
      metadata || {},
      methodDescriptor_CameraService_ObjectPointClouds,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.CameraServiceObjectPointCloudsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.CameraServiceObjectPointCloudsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.CameraServicePromiseClient.prototype.objectPointClouds =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.CameraService/ObjectPointClouds',
      request,
      metadata || {},
      methodDescriptor_CameraService_ObjectPointClouds);
};


module.exports = proto.proto.api.component.v1;

