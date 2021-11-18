// package: proto.api.component.v1
// file: proto/api/component/v1/arm.proto

var proto_api_component_v1_arm_pb = require("../../../../proto/api/component/v1/arm_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ArmService = (function () {
  function ArmService() {}
  ArmService.serviceName = "proto.api.component.v1.ArmService";
  return ArmService;
}());

ArmService.CurrentPosition = {
  methodName: "CurrentPosition",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceCurrentPositionRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceCurrentPositionResponse
};

ArmService.MoveToPosition = {
  methodName: "MoveToPosition",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceMoveToPositionResponse
};

ArmService.CurrentJointPositions = {
  methodName: "CurrentJointPositions",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceCurrentJointPositionsResponse
};

ArmService.MoveToJointPositions = {
  methodName: "MoveToJointPositions",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceMoveToJointPositionsResponse
};

ArmService.JointMoveDelta = {
  methodName: "JointMoveDelta",
  service: ArmService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaRequest,
  responseType: proto_api_component_v1_arm_pb.ArmServiceJointMoveDeltaResponse
};

exports.ArmService = ArmService;

function ArmServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ArmServiceClient.prototype.currentPosition = function currentPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmService.CurrentPosition, {
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

ArmServiceClient.prototype.currentJointPositions = function currentJointPositions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmService.CurrentJointPositions, {
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

ArmServiceClient.prototype.jointMoveDelta = function jointMoveDelta(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmService.JointMoveDelta, {
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

