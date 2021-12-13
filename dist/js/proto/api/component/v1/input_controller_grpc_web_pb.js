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
 *   !proto.proto.api.component.v1.InputControllerServiceControlsRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceControlsResponse>}
 */
const methodDescriptor_InputControllerService_Controls = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/Controls',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.InputControllerServiceControlsRequest,
  proto.proto.api.component.v1.InputControllerServiceControlsResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceControlsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceControlsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.InputControllerServiceControlsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceControlsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.controls =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/Controls',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_Controls,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceControlsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.InputControllerServiceControlsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.controls =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/Controls',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_Controls);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.InputControllerServiceLastEventsRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceLastEventsResponse>}
 */
const methodDescriptor_InputControllerService_LastEvents = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/LastEvents',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.InputControllerServiceLastEventsRequest,
  proto.proto.api.component.v1.InputControllerServiceLastEventsResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceLastEventsRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceLastEventsResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceLastEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.InputControllerServiceLastEventsResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceLastEventsResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.lastEvents =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/LastEvents',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_LastEvents,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceLastEventsRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.InputControllerServiceLastEventsResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.lastEvents =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/LastEvents',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_LastEvents);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.InputControllerServiceEventStreamRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceEventStreamResponse>}
 */
const methodDescriptor_InputControllerService_EventStream = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/EventStream',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.InputControllerServiceEventStreamRequest,
  proto.proto.api.component.v1.InputControllerServiceEventStreamResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceEventStreamRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceEventStreamResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceEventStreamRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.InputControllerServiceEventStreamResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceEventStreamResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.eventStream =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/EventStream',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_EventStream,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceEventStreamRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.InputControllerServiceEventStreamResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.eventStream =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/EventStream',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_EventStream);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.api.component.v1.InputControllerServiceInjectEventRequest,
 *   !proto.proto.api.component.v1.InputControllerServiceInjectEventResponse>}
 */
const methodDescriptor_InputControllerService_InjectEvent = new grpc.web.MethodDescriptor(
  '/proto.api.component.v1.InputControllerService/InjectEvent',
  grpc.web.MethodType.UNARY,
  proto.proto.api.component.v1.InputControllerServiceInjectEventRequest,
  proto.proto.api.component.v1.InputControllerServiceInjectEventResponse,
  /**
   * @param {!proto.proto.api.component.v1.InputControllerServiceInjectEventRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.api.component.v1.InputControllerServiceInjectEventResponse.deserializeBinary
);


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceInjectEventRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.RpcError, ?proto.proto.api.component.v1.InputControllerServiceInjectEventResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.api.component.v1.InputControllerServiceInjectEventResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.api.component.v1.InputControllerServiceClient.prototype.injectEvent =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/InjectEvent',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_InjectEvent,
      callback);
};


/**
 * @param {!proto.proto.api.component.v1.InputControllerServiceInjectEventRequest} request The
 *     request proto
 * @param {?Object<string, string>=} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.api.component.v1.InputControllerServiceInjectEventResponse>}
 *     Promise that resolves to the response
 */
proto.proto.api.component.v1.InputControllerServicePromiseClient.prototype.injectEvent =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.api.component.v1.InputControllerService/InjectEvent',
      request,
      metadata || {},
      methodDescriptor_InputControllerService_InjectEvent);
};


module.exports = proto.proto.api.component.v1;

