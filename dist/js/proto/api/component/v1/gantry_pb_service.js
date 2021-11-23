// package: proto.api.component.v1
// file: proto/api/component/v1/gantry.proto

var proto_api_component_v1_gantry_pb = require("../../../../proto/api/component/v1/gantry_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var GantryService = (function () {
  function GantryService() {}
  GantryService.serviceName = "proto.api.component.v1.GantryService";
  return GantryService;
}());

GantryService.CurrentPosition = {
  methodName: "CurrentPosition",
  service: GantryService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gantry_pb.GantryServiceCurrentPositionRequest,
  responseType: proto_api_component_v1_gantry_pb.GantryServiceCurrentPositionResponse
};

GantryService.MoveToPosition = {
  methodName: "MoveToPosition",
  service: GantryService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionRequest,
  responseType: proto_api_component_v1_gantry_pb.GantryServiceMoveToPositionResponse
};

GantryService.Lengths = {
  methodName: "Lengths",
  service: GantryService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_gantry_pb.GantryServiceLengthsRequest,
  responseType: proto_api_component_v1_gantry_pb.GantryServiceLengthsResponse
};

exports.GantryService = GantryService;

function GantryServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

GantryServiceClient.prototype.currentPosition = function currentPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GantryService.CurrentPosition, {
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

GantryServiceClient.prototype.lengths = function lengths(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(GantryService.Lengths, {
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

