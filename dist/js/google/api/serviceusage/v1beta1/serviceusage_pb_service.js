// package: google.api.serviceusage.v1beta1
// file: google/api/serviceusage/v1beta1/serviceusage.proto

var google_api_serviceusage_v1beta1_serviceusage_pb = require("../../../../google/api/serviceusage/v1beta1/serviceusage_pb");
var google_api_serviceusage_v1beta1_resources_pb = require("../../../../google/api/serviceusage/v1beta1/resources_pb");
var google_longrunning_operations_pb = require("../../../../google/longrunning/operations_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ServiceUsage = (function () {
  function ServiceUsage() {}
  ServiceUsage.serviceName = "google.api.serviceusage.v1beta1.ServiceUsage";
  return ServiceUsage;
}());

ServiceUsage.EnableService = {
  methodName: "EnableService",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.EnableServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.DisableService = {
  methodName: "DisableService",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.DisableServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.GetService = {
  methodName: "GetService",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.GetServiceRequest,
  responseType: google_api_serviceusage_v1beta1_resources_pb.Service
};

ServiceUsage.ListServices = {
  methodName: "ListServices",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesRequest,
  responseType: google_api_serviceusage_v1beta1_serviceusage_pb.ListServicesResponse
};

ServiceUsage.BatchEnableServices = {
  methodName: "BatchEnableServices",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.BatchEnableServicesRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.ListConsumerQuotaMetrics = {
  methodName: "ListConsumerQuotaMetrics",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsRequest,
  responseType: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerQuotaMetricsResponse
};

ServiceUsage.GetConsumerQuotaMetric = {
  methodName: "GetConsumerQuotaMetric",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaMetricRequest,
  responseType: google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric
};

ServiceUsage.GetConsumerQuotaLimit = {
  methodName: "GetConsumerQuotaLimit",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.GetConsumerQuotaLimitRequest,
  responseType: google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaLimit
};

ServiceUsage.CreateAdminOverride = {
  methodName: "CreateAdminOverride",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.CreateAdminOverrideRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.UpdateAdminOverride = {
  methodName: "UpdateAdminOverride",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.UpdateAdminOverrideRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.DeleteAdminOverride = {
  methodName: "DeleteAdminOverride",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.DeleteAdminOverrideRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.ListAdminOverrides = {
  methodName: "ListAdminOverrides",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesRequest,
  responseType: google_api_serviceusage_v1beta1_serviceusage_pb.ListAdminOverridesResponse
};

ServiceUsage.ImportAdminOverrides = {
  methodName: "ImportAdminOverrides",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.ImportAdminOverridesRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.CreateConsumerOverride = {
  methodName: "CreateConsumerOverride",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.CreateConsumerOverrideRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.UpdateConsumerOverride = {
  methodName: "UpdateConsumerOverride",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.UpdateConsumerOverrideRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.DeleteConsumerOverride = {
  methodName: "DeleteConsumerOverride",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.DeleteConsumerOverrideRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.ListConsumerOverrides = {
  methodName: "ListConsumerOverrides",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesRequest,
  responseType: google_api_serviceusage_v1beta1_serviceusage_pb.ListConsumerOverridesResponse
};

ServiceUsage.ImportConsumerOverrides = {
  methodName: "ImportConsumerOverrides",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.ImportConsumerOverridesRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceUsage.GenerateServiceIdentity = {
  methodName: "GenerateServiceIdentity",
  service: ServiceUsage,
  requestStream: false,
  responseStream: false,
  requestType: google_api_serviceusage_v1beta1_serviceusage_pb.GenerateServiceIdentityRequest,
  responseType: google_longrunning_operations_pb.Operation
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

ServiceUsageClient.prototype.listConsumerQuotaMetrics = function listConsumerQuotaMetrics(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.ListConsumerQuotaMetrics, {
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

ServiceUsageClient.prototype.getConsumerQuotaMetric = function getConsumerQuotaMetric(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.GetConsumerQuotaMetric, {
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

ServiceUsageClient.prototype.getConsumerQuotaLimit = function getConsumerQuotaLimit(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.GetConsumerQuotaLimit, {
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

ServiceUsageClient.prototype.createAdminOverride = function createAdminOverride(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.CreateAdminOverride, {
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

ServiceUsageClient.prototype.updateAdminOverride = function updateAdminOverride(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.UpdateAdminOverride, {
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

ServiceUsageClient.prototype.deleteAdminOverride = function deleteAdminOverride(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.DeleteAdminOverride, {
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

ServiceUsageClient.prototype.listAdminOverrides = function listAdminOverrides(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.ListAdminOverrides, {
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

ServiceUsageClient.prototype.importAdminOverrides = function importAdminOverrides(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.ImportAdminOverrides, {
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

ServiceUsageClient.prototype.createConsumerOverride = function createConsumerOverride(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.CreateConsumerOverride, {
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

ServiceUsageClient.prototype.updateConsumerOverride = function updateConsumerOverride(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.UpdateConsumerOverride, {
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

ServiceUsageClient.prototype.deleteConsumerOverride = function deleteConsumerOverride(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.DeleteConsumerOverride, {
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

ServiceUsageClient.prototype.listConsumerOverrides = function listConsumerOverrides(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.ListConsumerOverrides, {
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

ServiceUsageClient.prototype.importConsumerOverrides = function importConsumerOverrides(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.ImportConsumerOverrides, {
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

ServiceUsageClient.prototype.generateServiceIdentity = function generateServiceIdentity(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceUsage.GenerateServiceIdentity, {
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

