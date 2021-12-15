// package: proto.api.component.v1
// file: proto/api/component/v1/servo.proto

var proto_api_component_v1_servo_pb = require("../../../../proto/api/component/v1/servo_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ServoService = (function () {
  function ServoService() {}
  ServoService.serviceName = "proto.api.component.v1.ServoService";
  return ServoService;
}());

ServoService.Move = {
  methodName: "Move",
  service: ServoService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_servo_pb.ServoServiceMoveRequest,
  responseType: proto_api_component_v1_servo_pb.ServoServiceMoveResponse
};

ServoService.AngularOffset = {
  methodName: "AngularOffset",
  service: ServoService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_servo_pb.ServoServiceAngularOffsetRequest,
  responseType: proto_api_component_v1_servo_pb.ServoServiceAngularOffsetResponse
};

exports.ServoService = ServoService;

function ServoServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ServoServiceClient.prototype.move = function move(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServoService.Move, {
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

ServoServiceClient.prototype.angularOffset = function angularOffset(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServoService.AngularOffset, {
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

exports.ServoServiceClient = ServoServiceClient;

