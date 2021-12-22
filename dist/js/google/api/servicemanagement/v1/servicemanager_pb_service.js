// package: google.api.servicemanagement.v1
// file: google/api/servicemanagement/v1/servicemanager.proto

var google_api_servicemanagement_v1_servicemanager_pb = require("../../../../google/api/servicemanagement/v1/servicemanager_pb");
var google_api_service_pb = require("../../../../google/api/service_pb");
var google_api_servicemanagement_v1_resources_pb = require("../../../../google/api/servicemanagement/v1/resources_pb");
var google_longrunning_operations_pb = require("../../../../google/longrunning/operations_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var ServiceManager = (function () {
  function ServiceManager() {}
  ServiceManager.serviceName = "google.api.servicemanagement.v1.ServiceManager";
  return ServiceManager;
}());

ServiceManager.ListServices = {
  methodName: "ListServices",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.ListServicesRequest,
  responseType: google_api_servicemanagement_v1_servicemanager_pb.ListServicesResponse
};

ServiceManager.GetService = {
  methodName: "GetService",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.GetServiceRequest,
  responseType: google_api_servicemanagement_v1_resources_pb.ManagedService
};

ServiceManager.CreateService = {
  methodName: "CreateService",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceManager.DeleteService = {
  methodName: "DeleteService",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.DeleteServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceManager.UndeleteService = {
  methodName: "UndeleteService",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.UndeleteServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceManager.ListServiceConfigs = {
  methodName: "ListServiceConfigs",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsRequest,
  responseType: google_api_servicemanagement_v1_servicemanager_pb.ListServiceConfigsResponse
};

ServiceManager.GetServiceConfig = {
  methodName: "GetServiceConfig",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.GetServiceConfigRequest,
  responseType: google_api_service_pb.Service
};

ServiceManager.CreateServiceConfig = {
  methodName: "CreateServiceConfig",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceConfigRequest,
  responseType: google_api_service_pb.Service
};

ServiceManager.SubmitConfigSource = {
  methodName: "SubmitConfigSource",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.SubmitConfigSourceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceManager.ListServiceRollouts = {
  methodName: "ListServiceRollouts",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsRequest,
  responseType: google_api_servicemanagement_v1_servicemanager_pb.ListServiceRolloutsResponse
};

ServiceManager.GetServiceRollout = {
  methodName: "GetServiceRollout",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.GetServiceRolloutRequest,
  responseType: google_api_servicemanagement_v1_resources_pb.Rollout
};

ServiceManager.CreateServiceRollout = {
  methodName: "CreateServiceRollout",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.CreateServiceRolloutRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceManager.GenerateConfigReport = {
  methodName: "GenerateConfigReport",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportRequest,
  responseType: google_api_servicemanagement_v1_servicemanager_pb.GenerateConfigReportResponse
};

ServiceManager.EnableService = {
  methodName: "EnableService",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.EnableServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

ServiceManager.DisableService = {
  methodName: "DisableService",
  service: ServiceManager,
  requestStream: false,
  responseStream: false,
  requestType: google_api_servicemanagement_v1_servicemanager_pb.DisableServiceRequest,
  responseType: google_longrunning_operations_pb.Operation
};

exports.ServiceManager = ServiceManager;

function ServiceManagerClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

ServiceManagerClient.prototype.listServices = function listServices(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.ListServices, {
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

ServiceManagerClient.prototype.getService = function getService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.GetService, {
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

ServiceManagerClient.prototype.createService = function createService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.CreateService, {
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

ServiceManagerClient.prototype.deleteService = function deleteService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.DeleteService, {
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

ServiceManagerClient.prototype.undeleteService = function undeleteService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.UndeleteService, {
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

ServiceManagerClient.prototype.listServiceConfigs = function listServiceConfigs(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.ListServiceConfigs, {
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

ServiceManagerClient.prototype.getServiceConfig = function getServiceConfig(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.GetServiceConfig, {
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

ServiceManagerClient.prototype.createServiceConfig = function createServiceConfig(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.CreateServiceConfig, {
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

ServiceManagerClient.prototype.submitConfigSource = function submitConfigSource(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.SubmitConfigSource, {
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

ServiceManagerClient.prototype.listServiceRollouts = function listServiceRollouts(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.ListServiceRollouts, {
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

ServiceManagerClient.prototype.getServiceRollout = function getServiceRollout(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.GetServiceRollout, {
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

ServiceManagerClient.prototype.createServiceRollout = function createServiceRollout(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.CreateServiceRollout, {
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

ServiceManagerClient.prototype.generateConfigReport = function generateConfigReport(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.GenerateConfigReport, {
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

ServiceManagerClient.prototype.enableService = function enableService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.EnableService, {
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

ServiceManagerClient.prototype.disableService = function disableService(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(ServiceManager.DisableService, {
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

exports.ServiceManagerClient = ServiceManagerClient;

