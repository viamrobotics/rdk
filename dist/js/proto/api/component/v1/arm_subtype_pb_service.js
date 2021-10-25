// package: proto.api.component.v1
// file: proto/api/component/v1/arm_subtype.proto

var proto_api_component_v1_arm_subtype_pb = require("../../../../proto/api/component/v1/arm_subtype_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ArmSubtypeService = (function () {
  function ArmSubtypeService() {}
  ArmSubtypeService.serviceName = "proto.api.component.v1.ArmSubtypeService";
  return ArmSubtypeService;
}());

ArmSubtypeService.CurrentPosition = {
  methodName: "CurrentPosition",
  service: ArmSubtypeService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_subtype_pb.CurrentPositionRequest,
  responseType: proto_api_component_v1_arm_subtype_pb.CurrentPositionResponse
};

ArmSubtypeService.MoveToPosition = {
  methodName: "MoveToPosition",
  service: ArmSubtypeService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_subtype_pb.MoveToPositionRequest,
  responseType: proto_api_component_v1_arm_subtype_pb.MoveToPositionResponse
};

ArmSubtypeService.CurrentJointPositions = {
  methodName: "CurrentJointPositions",
  service: ArmSubtypeService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsRequest,
  responseType: proto_api_component_v1_arm_subtype_pb.CurrentJointPositionsResponse
};

ArmSubtypeService.MoveToJointPositions = {
  methodName: "MoveToJointPositions",
  service: ArmSubtypeService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsRequest,
  responseType: proto_api_component_v1_arm_subtype_pb.MoveToJointPositionsResponse
};

ArmSubtypeService.JointMoveDelta = {
  methodName: "JointMoveDelta",
  service: ArmSubtypeService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_arm_subtype_pb.JointMoveDeltaRequest,
  responseType: proto_api_component_v1_arm_subtype_pb.JointMoveDeltaResponse
};

exports.ArmSubtypeService = ArmSubtypeService;

function ArmSubtypeServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ArmSubtypeServiceClient.prototype.currentPosition = function currentPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmSubtypeService.CurrentPosition, {
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

ArmSubtypeServiceClient.prototype.moveToPosition = function moveToPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmSubtypeService.MoveToPosition, {
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

ArmSubtypeServiceClient.prototype.currentJointPositions = function currentJointPositions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmSubtypeService.CurrentJointPositions, {
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

ArmSubtypeServiceClient.prototype.moveToJointPositions = function moveToJointPositions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmSubtypeService.MoveToJointPositions, {
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

ArmSubtypeServiceClient.prototype.jointMoveDelta = function jointMoveDelta(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ArmSubtypeService.JointMoveDelta, {
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

exports.ArmSubtypeServiceClient = ArmSubtypeServiceClient;

