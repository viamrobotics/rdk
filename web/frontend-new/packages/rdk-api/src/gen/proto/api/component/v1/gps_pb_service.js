// package: proto.api.component.v1
// file: proto/api/component/v1/gps.proto

var proto_api_component_v1_gps_pb = require("../../../../proto/api/component/v1/gps_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var GPSService = (function () {
  function GPSService() {}
  GPSService.serviceName = "proto.api.component.v1.GPSService";
  return GPSService;
}());

GPSService.ReadLocation = {
  methodName: "ReadLocation",
  service: GPSService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gps_pb.GPSServiceReadLocationRequest,
  responseType: proto_api_component_v1_gps_pb.GPSServiceReadLocationResponse
};

GPSService.ReadAltitude = {
  methodName: "ReadAltitude",
  service: GPSService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gps_pb.GPSServiceReadAltitudeRequest,
  responseType: proto_api_component_v1_gps_pb.GPSServiceReadAltitudeResponse
};

GPSService.ReadSpeed = {
  methodName: "ReadSpeed",
  service: GPSService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gps_pb.GPSServiceReadSpeedRequest,
  responseType: proto_api_component_v1_gps_pb.GPSServiceReadSpeedResponse
};

exports.GPSService = GPSService;

function GPSServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

GPSServiceClient.prototype.readLocation = function readLocation(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GPSService.ReadLocation, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

GPSServiceClient.prototype.readAltitude = function readAltitude(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GPSService.ReadAltitude, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

GPSServiceClient.prototype.readSpeed = function readSpeed(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GPSService.ReadSpeed, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onEnd: function (response) {
      if (callback) {
        if (response.status !== grpc.Code.OK) {
          var err = new Error(response.statusMessage);
          err.code = response.status;
          err.metadata = response.trailers;
          callback(err, null);
        } else {
          callback(null, response.message);
        }
      }
    }
  });
  return {
    cancel: function () {
      callback = null;
      client.close();
    }
  };
};

exports.GPSServiceClient = GPSServiceClient;

