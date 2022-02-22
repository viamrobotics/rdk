// package: proto.api.service.v1
// file: proto/api/service/v1/metadata.proto

var proto_api_service_v1_metadata_pb = require("../../../../proto/api/service/v1/metadata_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var MetadataService = (function () {
  function MetadataService() {}
  MetadataService.serviceName = "proto.api.service.v1.MetadataService";
  return MetadataService;
}());

MetadataService.Resources = {
  methodName: "Resources",
  service: MetadataService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_service_v1_metadata_pb.ResourcesRequest,
  responseType: proto_api_service_v1_metadata_pb.ResourcesResponse
};

exports.MetadataService = MetadataService;

function MetadataServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

MetadataServiceClient.prototype.resources = function resources(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MetadataService.Resources, {
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

exports.MetadataServiceClient = MetadataServiceClient;

