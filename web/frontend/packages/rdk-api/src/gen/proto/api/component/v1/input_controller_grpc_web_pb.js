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

var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js')
const proto = {};
proto.proto = {};
proto.proto.api = {};
proto.proto.api.component = {};
proto.proto.api.component.v1 = require('./input_controller_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?grpc.web.ClientOptions} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.api.component.v1.InputControllerServiceClient =
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
proto.proto.api.component.v1.InputControllerServicePromiseClient =
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
 *   !proto.proto.api.component.v1.InputControllerServiceGetControlsRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceGetControlsResponse>}
 */
const methodDescriptor_InputControllerService_GetControls = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/GetControls',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.InputControllerServiceGetControlsRequest,
  proto.proto.api.component.v1.InputControllerServiceGetControlsResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceGetControlsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceGetControlsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceGetControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.InputControllerServiceGetControlsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceGetControlsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.getControls =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/GetControls',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_GetControls,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceGetControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.InputControllerServiceGetControlsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.getControls =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/GetControls',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_GetControls);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.InputControllerServiceGetEventsRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceGetEventsResponse>}
 */
const methodDescriptor_InputControllerService_GetEvents = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/GetEvents',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.InputControllerServiceGetEventsRequest,
  proto.proto.api.component.v1.InputControllerServiceGetEventsResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceGetEventsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceGetEventsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceGetEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.InputControllerServiceGetEventsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceGetEventsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.getEvents =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/GetEvents',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_GetEvents,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceGetEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.InputControllerServiceGetEventsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.getEvents =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/GetEvents',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_GetEvents);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.InputControllerServiceStreamEventsRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceStreamEventsResponse>}
 */
const methodDescriptor_InputControllerService_StreamEvents = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/StreamEvents',
  grpc.web.MethodType.SERVER_STREAMING,
  proto.proto.api.component.v1.InputControllerServiceStreamEventsRequest,
  proto.proto.api.component.v1.InputControllerServiceStreamEventsResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceStreamEventsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceStreamEventsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceStreamEventsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceStreamEventsResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.streamEvents =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/StreamEvents',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_StreamEvents);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceStreamEventsRequest} request The request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceStreamEventsResponse>}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.streamEvents =
    function(request, metadata) {
  return this.client_.serverStreaming(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/StreamEvents',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_StreamEvents);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.InputControllerServiceTriggerEventRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceTriggerEventResponse>}
 */
const methodDescriptor_InputControllerService_TriggerEvent = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/TriggerEvent',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.InputControllerServiceTriggerEventRequest,
  proto.proto.api.component.v1.InputControllerServiceTriggerEventResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceTriggerEventRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceTriggerEventResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceTriggerEventRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.InputControllerServiceTriggerEventResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceTriggerEventResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.triggerEvent =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/TriggerEvent',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_TriggerEvent,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceTriggerEventRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.InputControllerServiceTriggerEventResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.triggerEvent =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/TriggerEvent',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_TriggerEvent);
};


module.exports = proto.proto.api.component.v1;

