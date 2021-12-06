// package: proto.api.component.v1
// file: proto/api/component/v1/imu.proto

var proto_api_component_v1_imu_pb = require("../../../../proto/api/component/v1/imu_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var IMUService = (function () {
  function IMUService() {}
  IMUService.serviceName = "proto.api.component.v1.IMUService";
  return IMUService;
}());

IMUService.AngularVelocity = {
  methodName: "AngularVelocity",
  service: IMUService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_imu_pb.IMUServiceAngularVelocityRequest,
  responseType: proto_api_component_v1_imu_pb.IMUServiceAngularVelocityResponse
};

IMUService.Orientation = {
  methodName: "Orientation",
  service: IMUService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_imu_pb.IMUServiceOrientationRequest,
  responseType: proto_api_component_v1_imu_pb.IMUServiceOrientationResponse
};

exports.IMUService = IMUService;

function IMUServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

IMUServiceClient.prototype.angularVelocity = function angularVelocity(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(IMUService.AngularVelocity, {
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

IMUServiceClient.prototype.orientation = function orientation(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(IMUService.Orientation, {
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

