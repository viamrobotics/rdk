// package: proto.api.component.v1
// file: proto/api/component/v1/imu.proto

var proto_api_component_v1_imu_pb = require("../../../../proto/api/component/v1/imu_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var IMUService = (function () {
  function IMUService() {}
  IMUService.serviceName = "proto.api.component.v1.IMUService";
  return IMUService;
}());

IMUService.ReadAngularVelocity = {
  methodName: "ReadAngularVelocity",
  service: IMUService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_imu_pb.IMUServiceReadAngularVelocityRequest,
  responseType: proto_api_component_v1_imu_pb.IMUServiceReadAngularVelocityResponse
};

IMUService.ReadOrientation = {
  methodName: "ReadOrientation",
  service: IMUService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_imu_pb.IMUServiceReadOrientationRequest,
  responseType: proto_api_component_v1_imu_pb.IMUServiceReadOrientationResponse
};

exports.IMUService = IMUService;

function IMUServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

IMUServiceClient.prototype.readAngularVelocity = function readAngularVelocity(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(IMUService.ReadAngularVelocity, {
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

IMUServiceClient.prototype.readOrientation = function readOrientation(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(IMUService.ReadOrientation, {
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

exports.IMUServiceClient = IMUServiceClient;

