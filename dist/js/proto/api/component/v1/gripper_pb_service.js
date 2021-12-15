// package: proto.api.component.v1
// file: proto/api/component/v1/gripper.proto

var proto_api_component_v1_gripper_pb = require("../../../../proto/api/component/v1/gripper_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var GripperService = (function () {
  function GripperService() {}
  GripperService.serviceName = "proto.api.component.v1.GripperService";
  return GripperService;
}());

GripperService.Open = {
  methodName: "Open",
  service: GripperService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gripper_pb.GripperServiceOpenRequest,
  responseType: proto_api_component_v1_gripper_pb.GripperServiceOpenResponse
};

GripperService.Grab = {
  methodName: "Grab",
  service: GripperService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gripper_pb.GripperServiceGrabRequest,
  responseType: proto_api_component_v1_gripper_pb.GripperServiceGrabResponse
};

exports.GripperService = GripperService;

function GripperServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

GripperServiceClient.prototype.open = function open(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GripperService.Open, {
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

GripperServiceClient.prototype.grab = function grab(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GripperService.Grab, {
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

exports.GripperServiceClient = GripperServiceClient;

