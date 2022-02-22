// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/service_controller.proto

var google_api_servicecontrol_v1_service_controller_pb = require("../../../../google/api/servicecontrol/v1/service_controller_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ServiceController = (function () {
  function ServiceController() {}
  ServiceController.serviceName = "google.api.servicecontrol.v1.ServiceController";
  return ServiceController;
}());

ServiceController.Check = {
  methodName: "Check",
  service: ServiceController,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicecontrol_v1_service_controller_pb.CheckRequest,
  responseType: google_api_servicecontrol_v1_service_controller_pb.CheckResponse
};

ServiceController.Report = {
  methodName: "Report",
  service: ServiceController,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicecontrol_v1_service_controller_pb.ReportRequest,
  responseType: google_api_servicecontrol_v1_service_controller_pb.ReportResponse
};

exports.ServiceController = ServiceController;

function ServiceControllerClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ServiceControllerClient.prototype.check = function check(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceController.Check, {
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

ServiceControllerClient.prototype.report = function report(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceController.Report, {
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

exports.ServiceControllerClient = ServiceControllerClient;

