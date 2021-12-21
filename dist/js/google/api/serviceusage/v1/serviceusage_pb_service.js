// package: google.api.serviceusage.v1
// file: google/api/serviceusage/v1/serviceusage.proto

var google_api_serviceusage_v1_serviceusage_pb = require("../../../../google/api/serviceusage/v1/serviceusage_pb");
var google_api_serviceusage_v1_resources_pb = require("../../../../google/api/serviceusage/v1/resources_pb");
var google_longrunning_operations_pb = require("../../../../google/longrunning/operations_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ServiceUsage = (function () {
  function ServiceUsage() {}
  ServiceUsage.serviceName = "google.api.serviceusage.v1.ServiceUsage";
  return ServiceUsage;
}());

ServiceUsage.EnableService = {
  methodName: "EnableService",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1_serviceusage_pb.EnableServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.DisableService = {
  methodName: "DisableService",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1_serviceusage_pb.DisableServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.GetService = {
  methodName: "GetService",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1_serviceusage_pb.GetServiceRequest,
  responseType: google_api_serviceusage_v1_resources_pb.Service
};

ServiceUsage.ListServices = {
  methodName: "ListServices",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1_serviceusage_pb.ListServicesRequest,
  responseType: google_api_serviceusage_v1_serviceusage_pb.ListServicesResponse
};

ServiceUsage.BatchEnableServices = {
  methodName: "BatchEnableServices",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1_serviceusage_pb.BatchEnableServicesRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.BatchGetServices = {
  methodName: "BatchGetServices",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesRequest,
  responseType: google_api_serviceusage_v1_serviceusage_pb.BatchGetServicesResponse
};

exports.ServiceUsage = ServiceUsage;

function ServiceUsageClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ServiceUsageClient.prototype.enableService = function enableService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.EnableService, {
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

ServiceUsageClient.prototype.disableService = function disableService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.DisableService, {
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

ServiceUsageClient.prototype.getService = function getService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.GetService, {
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

ServiceUsageClient.prototype.listServices = function listServices(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.ListServices, {
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

ServiceUsageClient.prototype.batchEnableServices = function batchEnableServices(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.BatchEnableServices, {
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

ServiceUsageClient.prototype.batchGetServices = function batchGetServices(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.BatchGetServices, {
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

exports.ServiceUsageClient = ServiceUsageClient;

