// package: proto.api.component.v1
// file: proto/api/component/v1/motor.proto

var proto_api_component_v1_motor_pb = require("../../../../proto/api/component/v1/motor_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var MotorService = (function () {
  function MotorService() {}
  MotorService.serviceName = "proto.api.component.v1.MotorService";
  return MotorService;
}());

MotorService.GetPIDConfig = {
  methodName: "GetPIDConfig",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceGetPIDConfigRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceGetPIDConfigResponse
};

MotorService.SetPIDConfig = {
  methodName: "SetPIDConfig",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceSetPIDConfigRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceSetPIDConfigResponse
};

MotorService.PIDStep = {
  methodName: "PIDStep",
  service: MotorService,
  requestStream: false,
  responseStream: true,
  requestType: proto_api_component_v1_motor_pb.MotorServicePIDStepRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServicePIDStepResponse
};

MotorService.SetPower = {
  methodName: "SetPower",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceSetPowerRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceSetPowerResponse
};

MotorService.Go = {
  methodName: "Go",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceGoRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceGoResponse
};

MotorService.GoFor = {
  methodName: "GoFor",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceGoForRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceGoForResponse
};

MotorService.GoTo = {
  methodName: "GoTo",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceGoToRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceGoToResponse
};

MotorService.GoTillStop = {
  methodName: "GoTillStop",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceGoTillStopRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceGoTillStopResponse
};

MotorService.ResetZeroPosition = {
  methodName: "ResetZeroPosition",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceResetZeroPositionResponse
};

MotorService.Position = {
  methodName: "Position",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServicePositionRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServicePositionResponse
};

MotorService.PositionSupported = {
  methodName: "PositionSupported",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServicePositionSupportedRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServicePositionSupportedResponse
};

MotorService.Stop = {
  methodName: "Stop",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceStopRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceStopResponse
};

MotorService.IsOn = {
  methodName: "IsOn",
  service: MotorService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_motor_pb.MotorServiceIsOnRequest,
  responseType: proto_api_component_v1_motor_pb.MotorServiceIsOnResponse
};

exports.MotorService = MotorService;

function MotorServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

MotorServiceClient.prototype.getPIDConfig = function getPIDConfig(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.GetPIDConfig, {
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

MotorServiceClient.prototype.setPIDConfig = function setPIDConfig(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.SetPIDConfig, {
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

MotorServiceClient.prototype.pIDStep = function pIDStep(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(MotorService.PIDStep, {
    request: requestMessage,
    host: this.serviceHost,
    metadata: metadata,
    transport: this.options.transport,
    debug: this.options.debug,
    onMessage: function (responseMessage) {
      listeners.data.forEach(function (handler) {
        handler(responseMessage);
      });
    },
    onEnd: function (status, statusMessage, trailers) {
      listeners.status.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners.end.forEach(function (handler) {
        handler({ code: status, details: statusMessage, metadata: trailers });
      });
      listeners = null;
    }
  });
  return {
    on: function (type, handler) {
      listeners[type].push(handler);
      return this;
    },
    cancel: function () {
      listeners = null;
      client.close();
    }
  };
};

MotorServiceClient.prototype.setPower = function setPower(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.SetPower, {
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

MotorServiceClient.prototype.go = function go(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.Go, {
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

MotorServiceClient.prototype.goFor = function goFor(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.GoFor, {
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

MotorServiceClient.prototype.goTo = function goTo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.GoTo, {
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

MotorServiceClient.prototype.goTillStop = function goTillStop(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.GoTillStop, {
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

MotorServiceClient.prototype.resetZeroPosition = function resetZeroPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.ResetZeroPosition, {
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

MotorServiceClient.prototype.position = function position(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.Position, {
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

MotorServiceClient.prototype.positionSupported = function positionSupported(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.PositionSupported, {
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

MotorServiceClient.prototype.stop = function stop(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.Stop, {
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

MotorServiceClient.prototype.isOn = function isOn(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(MotorService.IsOn, {
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

exports.MotorServiceClient = MotorServiceClient;

