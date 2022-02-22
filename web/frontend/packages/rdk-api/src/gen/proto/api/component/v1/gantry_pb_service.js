// package: proto.api.component.v1
// file: proto/api/component/v1/gantry.proto

var proto_api_component_v1_gantry_pb = require("../../../../proto/api/component/v1/gantry_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var GantryService = (function () {
  function GantryService() {}
  GantryService.serviceName = "proto.api.component.v1.GantryService";
  return GantryService;
}());

GantryService.GetPosition = {
  methodName: "GetPosition",
  service: GantryService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gantry_pb.GantryServiceGetPositionRequest,
  responseType: proto_api_component_v1_gantry_pb.GantryServiceGetPositionResponse
};

GantryService.MoveToPosition = {
  methodName: "MoveToPosition",
  service: GantryService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionRequest,
  responseType: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionResponse
};

GantryService.GetLengths = {
  methodName: "GetLengths",
  service: GantryService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gantry_pb.GantryServiceGetLengthsRequest,
  responseType: proto_api_component_v1_gantry_pb.GantryServiceGetLengthsResponse
};

exports.GantryService = GantryService;

function GantryServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

GantryServiceClient.prototype.getPosition = function getPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GantryService.GetPosition, {
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

GantryServiceClient.prototype.moveToPosition = function moveToPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GantryService.MoveToPosition, {
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

GantryServiceClient.prototype.getLengths = function getLengths(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GantryService.GetLengths, {
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

exports.GantryServiceClient = GantryServiceClient;

