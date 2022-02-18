// package: proto.api.component.v1
// file: proto/api/component/v1/arm.proto

var proto_api_component_v1_arm_pb = require("../../../../proto/api/component/v1/arm_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ArmService = (function () {
  function ArmService() {}
  ArmService.serviceName = "proto.api.component.v1.ArmService";
  return ArmService;
}());

ArmService.GetEndPosition = {
  methodName: "GetEndPosition",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceGetEndPositionRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceGetEndPositionResponse
};

ArmService.MoveToPosition = {
  methodName: "MoveToPosition",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionResponse
};

ArmService.GetJointPositions = {
  methodName: "GetJointPositions",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceGetJointPositionsResponse
};

ArmService.MoveToJointPositions = {
  methodName: "MoveToJointPositions",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsResponse
};

exports.ArmService = ArmService;

function ArmServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ArmServiceClient.prototype.getEndPosition = function getEndPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmService.GetEndPosition, {
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

ArmServiceClient.prototype.moveToPosition = function moveToPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmService.MoveToPosition, {
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

ArmServiceClient.prototype.getJointPositions = function getJointPositions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmService.GetJointPositions, {
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

ArmServiceClient.prototype.moveToJointPositions = function moveToJointPositions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmService.MoveToJointPositions, {
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

exports.ArmServiceClient = ArmServiceClient;

