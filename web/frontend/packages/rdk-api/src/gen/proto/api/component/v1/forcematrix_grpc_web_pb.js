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
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.component = {};
proto.proto.api.component.v1 = require('./forcematrix_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.ForceMatrixServiceClient =
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
proto.proto.api.component.v1.ForceMatrixServicePromiseClient =
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
 *   !proto.proto.api.component.v1.ForceMatrixServiceReadMatrixRequest,
 *   !proto.proto.api.component.v1.ForceMatrixServiceReadMatrixResponse>}
 */
const methodDescriptor_ForceMatrixService_ReadMatrix = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ForceMatrixService/ReadMatrix',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ForceMatrixServiceReadMatrixRequest,
  proto.proto.api.component.v1.ForceMatrixServiceReadMatrixResponse,
  /**
   * @param {!proto.proto.api.component.v1.ForceMatrixServiceReadMatrixRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ForceMatrixServiceReadMatrixResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ForceMatrixServiceReadMatrixRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ForceMatrixServiceReadMatrixResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ForceMatrixServiceReadMatrixResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ForceMatrixServiceClient.prototype.readMatrix =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ForceMatrixService/ReadMatrix',
      request,
      metadata || {},
      methodDescriptor_ForceMatrixService_ReadMatrix,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ForceMatrixServiceReadMatrixRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ForceMatrixServiceReadMatrixResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ForceMatrixServicePromiseClient.prototype.readMatrix =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ForceMatrixService/ReadMatrix',
      request,
      metadata || {},
      methodDescriptor_ForceMatrixService_ReadMatrix);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.ForceMatrixServiceDetectSlipRequest,
 *   !proto.proto.api.component.v1.ForceMatrixServiceDetectSlipResponse>}
 */
const methodDescriptor_ForceMatrixService_DetectSlip = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.ForceMatrixService/DetectSlip',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.ForceMatrixServiceDetectSlipRequest,
  proto.proto.api.component.v1.ForceMatrixServiceDetectSlipResponse,
  /**
   * @param {!proto.proto.api.component.v1.ForceMatrixServiceDetectSlipRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.ForceMatrixServiceDetectSlipResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.ForceMatrixServiceDetectSlipRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.ForceMatrixServiceDetectSlipResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.ForceMatrixServiceDetectSlipResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.ForceMatrixServiceClient.prototype.detectSlip =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.ForceMatrixService/DetectSlip',
      request,
      metadata || {},
      methodDescriptor_ForceMatrixService_DetectSlip,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.ForceMatrixServiceDetectSlipRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.ForceMatrixServiceDetectSlipResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.ForceMatrixServicePromiseClient.prototype.detectSlip =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.ForceMatrixService/DetectSlip',
      request,
      metadata || {},
      methodDescriptor_ForceMatrixService_DetectSlip);
};


module.exports = proto.proto.api.component.v1;

