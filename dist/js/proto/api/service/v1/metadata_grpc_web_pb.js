/**
 * @fileoverview gRPC-Web generated client stub for proto.api.service.v1
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

var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js')

var google_api_httpbody_pb = require('../../../../google/api/httpbody_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.service = {};
proto.proto.api.service.v1 = require('./metadata_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.service.v1.MetadataServiceClient =
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
proto.proto.api.service.v1.MetadataServicePromiseClient =
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
 *   !proto.proto.api.service.v1.ResourcesRequest,
 *   !proto.proto.api.service.v1.ResourcesResponse>}
 */
const methodDescriptor_MetadataService_Resources = new grpc.web.MethodDescriptor(
  '/proto.api.service.v1.MetadataService/Resources',
  grpc.web.MethodType.UNARY,
  proto.proto.api.service.v1.ResourcesRequest,
  proto.proto.api.service.v1.ResourcesResponse,
  /**
   * @param {!proto.proto.api.service.v1.ResourcesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.service.v1.ResourcesResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.api.service.v1.ResourcesRequest,
 *   !proto.proto.api.service.v1.ResourcesResponse>}
 */
const methodInfo_MetadataService_Resources = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.api.service.v1.ResourcesResponse,
  /**
   * @param {!proto.proto.api.service.v1.ResourcesRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.service.v1.ResourcesResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.service.v1.ResourcesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.api.service.v1.ResourcesResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.service.v1.ResourcesResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.service.v1.MetadataServiceClient.prototype.resources =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.service.v1.MetadataService/Resources',
      request,
      metadata || {},
      methodDescriptor_MetadataService_Resources,
      callback);
};


/**
 * @param {!proto.proto.api.service.v1.ResourcesRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.service.v1.ResourcesResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.service.v1.MetadataServicePromiseClient.prototype.resources =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.service.v1.MetadataService/Resources',
      request,
      metadata || {},
      methodDescriptor_MetadataService_Resources);
};


module.exports = proto.proto.api.service.v1;

