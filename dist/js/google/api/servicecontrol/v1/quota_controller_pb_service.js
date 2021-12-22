// package: google.api.servicecontrol.v1
// file: google/api/servicecontrol/v1/quota_controller.proto

var google_api_servicecontrol_v1_quota_controller_pb = require("../../../../google/api/servicecontrol/v1/quota_controller_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var QuotaController = (function () {
  function QuotaController() {}
  QuotaController.serviceName = "google.api.servicecontrol.v1.QuotaController";
  return QuotaController;
}());

QuotaController.AllocateQuota = {
  methodName: "AllocateQuota",
  service: QuotaController,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaRequest,
  responseType: google_api_servicecontrol_v1_quota_controller_pb.AllocateQuotaResponse
};

exports.QuotaController = QuotaController;

function QuotaControllerClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

QuotaControllerClient.prototype.allocateQuota = function allocateQuota(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(QuotaController.AllocateQuota, {
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

exports.QuotaControllerClient = QuotaControllerClient;

