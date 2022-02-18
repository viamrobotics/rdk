// package: proto.api.component.v1
// file: proto/api/component/v1/forcematrix.proto

var proto_api_component_v1_forcematrix_pb = require("../../../../proto/api/component/v1/forcematrix_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ForceMatrixService = (function () {
  function ForceMatrixService() {}
  ForceMatrixService.serviceName = "proto.api.component.v1.ForceMatrixService";
  return ForceMatrixService;
}());

ForceMatrixService.ReadMatrix = {
  methodName: "ReadMatrix",
  service: ForceMatrixService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixRequest,
  responseType: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceReadMatrixResponse
};

ForceMatrixService.DetectSlip = {
  methodName: "DetectSlip",
  service: ForceMatrixService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipRequest,
  responseType: proto_api_component_v1_forcematrix_pb.ForceMatrixServiceDetectSlipResponse
};

exports.ForceMatrixService = ForceMatrixService;

function ForceMatrixServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ForceMatrixServiceClient.prototype.readMatrix = function readMatrix(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ForceMatrixService.ReadMatrix, {
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

ForceMatrixServiceClient.prototype.detectSlip = function detectSlip(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ForceMatrixService.DetectSlip, {
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

exports.ForceMatrixServiceClient = ForceMatrixServiceClient;

