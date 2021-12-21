// package: google.api.expr.conformance.v1alpha1
// file: google/api/expr/conformance/v1alpha1/conformance_service.proto

var google_api_expr_conformance_v1alpha1_conformance_service_pb = require("../../../../../google/api/expr/conformance/v1alpha1/conformance_service_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ConformanceService = (function () {
  function ConformanceService() {}
  ConformanceService.serviceName = "google.api.expr.conformance.v1alpha1.ConformanceService";
  return ConformanceService;
}());

ConformanceService.Parse = {
  methodName: "Parse",
  service: ConformanceService,
  requestStream: false,
  responseStream: false,
  requestType: google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseRequest,
  responseType: google_api_expr_conformance_v1alpha1_conformance_service_pb.ParseResponse
};

ConformanceService.Check = {
  methodName: "Check",
  service: ConformanceService,
  requestStream: false,
  responseStream: false,
  requestType: google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckRequest,
  responseType: google_api_expr_conformance_v1alpha1_conformance_service_pb.CheckResponse
};

ConformanceService.Eval = {
  methodName: "Eval",
  service: ConformanceService,
  requestStream: false,
  responseStream: false,
  requestType: google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalRequest,
  responseType: google_api_expr_conformance_v1alpha1_conformance_service_pb.EvalResponse
};

exports.ConformanceService = ConformanceService;

function ConformanceServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ConformanceServiceClient.prototype.parse = function parse(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ConformanceService.Parse, {
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

ConformanceServiceClient.prototype.check = function check(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ConformanceService.Check, {
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

ConformanceServiceClient.prototype.eval = function eval(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ConformanceService.Eval, {
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

exports.ConformanceServiceClient = ConformanceServiceClient;

