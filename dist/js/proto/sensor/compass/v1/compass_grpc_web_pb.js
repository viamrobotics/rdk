/**
 * @fileoverview gRPC-Web generated client stub for proto.sensor.compass.v1
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
proto.proto.sensor = {};
proto.proto.sensor.compass = {};
proto.proto.sensor.compass.v1 = require('./compass_pb.js');

/**
 * @param {string} hostname
 * @param {?Object} credentials
 * @param {?Object} options
 * @constructor
 * @struct
 * @final
 */
proto.proto.sensor.compass.v1.CompassServiceClient =
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
proto.proto.sensor.compass.v1.CompassServicePromiseClient =
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
 *   !proto.proto.sensor.compass.v1.HeadingRequest,
 *   !proto.proto.sensor.compass.v1.HeadingResponse>}
 */
const methodDescriptor_CompassService_Heading = new grpc.web.MethodDescriptor(
  '/proto.sensor.compass.v1.CompassService/Heading',
  grpc.web.MethodType.UNARY,
  proto.proto.sensor.compass.v1.HeadingRequest,
  proto.proto.sensor.compass.v1.HeadingResponse,
  /**
   * @param {!proto.proto.sensor.compass.v1.HeadingRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.sensor.compass.v1.HeadingResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.sensor.compass.v1.HeadingRequest,
 *   !proto.proto.sensor.compass.v1.HeadingResponse>}
 */
const methodInfo_CompassService_Heading = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.sensor.compass.v1.HeadingResponse,
  /**
   * @param {!proto.proto.sensor.compass.v1.HeadingRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.sensor.compass.v1.HeadingResponse.deserializeBinary
);


/**
 * @param {!proto.proto.sensor.compass.v1.HeadingRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.sensor.compass.v1.HeadingResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.sensor.compass.v1.HeadingResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.sensor.compass.v1.CompassServiceClient.prototype.heading =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.sensor.compass.v1.CompassService/Heading',
      request,
      metadata || {},
      methodDescriptor_CompassService_Heading,
      callback);
};


/**
 * @param {!proto.proto.sensor.compass.v1.HeadingRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.sensor.compass.v1.HeadingResponse>}
 *     Promise that resolves to the response
 */
proto.proto.sensor.compass.v1.CompassServicePromiseClient.prototype.heading =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.sensor.compass.v1.CompassService/Heading',
      request,
      metadata || {},
      methodDescriptor_CompassService_Heading);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.sensor.compass.v1.StartCalibrationRequest,
 *   !proto.proto.sensor.compass.v1.StartCalibrationResponse>}
 */
const methodDescriptor_CompassService_StartCalibration = new grpc.web.MethodDescriptor(
  '/proto.sensor.compass.v1.CompassService/StartCalibration',
  grpc.web.MethodType.UNARY,
  proto.proto.sensor.compass.v1.StartCalibrationRequest,
  proto.proto.sensor.compass.v1.StartCalibrationResponse,
  /**
   * @param {!proto.proto.sensor.compass.v1.StartCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.sensor.compass.v1.StartCalibrationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.sensor.compass.v1.StartCalibrationRequest,
 *   !proto.proto.sensor.compass.v1.StartCalibrationResponse>}
 */
const methodInfo_CompassService_StartCalibration = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.sensor.compass.v1.StartCalibrationResponse,
  /**
   * @param {!proto.proto.sensor.compass.v1.StartCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.sensor.compass.v1.StartCalibrationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.sensor.compass.v1.StartCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.sensor.compass.v1.StartCalibrationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.sensor.compass.v1.StartCalibrationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.sensor.compass.v1.CompassServiceClient.prototype.startCalibration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.sensor.compass.v1.CompassService/StartCalibration',
      request,
      metadata || {},
      methodDescriptor_CompassService_StartCalibration,
      callback);
};


/**
 * @param {!proto.proto.sensor.compass.v1.StartCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.sensor.compass.v1.StartCalibrationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.sensor.compass.v1.CompassServicePromiseClient.prototype.startCalibration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.sensor.compass.v1.CompassService/StartCalibration',
      request,
      metadata || {},
      methodDescriptor_CompassService_StartCalibration);
};


/**
 * @const
 * @type {!grpc.web.MethodDescriptor<
 *   !proto.proto.sensor.compass.v1.StopCalibrationRequest,
 *   !proto.proto.sensor.compass.v1.StopCalibrationResponse>}
 */
const methodDescriptor_CompassService_StopCalibration = new grpc.web.MethodDescriptor(
  '/proto.sensor.compass.v1.CompassService/StopCalibration',
  grpc.web.MethodType.UNARY,
  proto.proto.sensor.compass.v1.StopCalibrationRequest,
  proto.proto.sensor.compass.v1.StopCalibrationResponse,
  /**
   * @param {!proto.proto.sensor.compass.v1.StopCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.sensor.compass.v1.StopCalibrationResponse.deserializeBinary
);


/**
 * @const
 * @type {!grpc.web.AbstractClientBase.MethodInfo<
 *   !proto.proto.sensor.compass.v1.StopCalibrationRequest,
 *   !proto.proto.sensor.compass.v1.StopCalibrationResponse>}
 */
const methodInfo_CompassService_StopCalibration = new grpc.web.AbstractClientBase.MethodInfo(
  proto.proto.sensor.compass.v1.StopCalibrationResponse,
  /**
   * @param {!proto.proto.sensor.compass.v1.StopCalibrationRequest} request
   * @return {!Uint8Array}
   */
  function(request) {
    return request.serializeBinary();
  },
  proto.proto.sensor.compass.v1.StopCalibrationResponse.deserializeBinary
);


/**
 * @param {!proto.proto.sensor.compass.v1.StopCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @param {function(?grpc.web.Error, ?proto.proto.sensor.compass.v1.StopCalibrationResponse)}
 *     callback The callback function(error, response)
 * @return {!grpc.web.ClientReadableStream<!proto.proto.sensor.compass.v1.StopCalibrationResponse>|undefined}
 *     The XHR Node Readable Stream
 */
proto.proto.sensor.compass.v1.CompassServiceClient.prototype.stopCalibration =
    function(request, metadata, callback) {
  return this.client_.rpcCall(this.hostname_ +
      '/proto.sensor.compass.v1.CompassService/StopCalibration',
      request,
      metadata || {},
      methodDescriptor_CompassService_StopCalibration,
      callback);
};


/**
 * @param {!proto.proto.sensor.compass.v1.StopCalibrationRequest} request The
 *     request proto
 * @param {?Object<string, string>} metadata User defined
 *     call metadata
 * @return {!Promise<!proto.proto.sensor.compass.v1.StopCalibrationResponse>}
 *     Promise that resolves to the response
 */
proto.proto.sensor.compass.v1.CompassServicePromiseClient.prototype.stopCalibration =
    function(request, metadata) {
  return this.client_.unaryCall(this.hostname_ +
      '/proto.sensor.compass.v1.CompassService/StopCalibration',
      request,
      metadata || {},
      methodDescriptor_CompassService_StopCalibration);
};


module.exports = proto.proto.sensor.compass.v1;

