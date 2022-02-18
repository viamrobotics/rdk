// package: proto.api.v1
// file: proto/api/v1/robot.proto

var proto_api_v1_robot_pb = require("../../../proto/api/v1/robot_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var RobotService = (function () {
  function RobotService() {}
  RobotService.serviceName = "proto.api.v1.RobotService";
  return RobotService;
}());

RobotService.Status = {
  methodName: "Status",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.StatusRequest,
  responseType: proto_api_v1_robot_pb.StatusResponse
};

RobotService.StatusStream = {
  methodName: "StatusStream",
  service: RobotService,
  requestStream: false,
  responseStream: true,
  requestType: proto_api_v1_robot_pb.StatusStreamRequest,
  responseType: proto_api_v1_robot_pb.StatusStreamResponse
};

RobotService.Config = {
  methodName: "Config",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ConfigRequest,
  responseType: proto_api_v1_robot_pb.ConfigResponse
};

RobotService.DoAction = {
  methodName: "DoAction",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.DoActionRequest,
  responseType: proto_api_v1_robot_pb.DoActionResponse
};

RobotService.SensorReadings = {
  methodName: "SensorReadings",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.SensorReadingsRequest,
  responseType: proto_api_v1_robot_pb.SensorReadingsResponse
};

RobotService.ExecuteFunction = {
  methodName: "ExecuteFunction",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ExecuteFunctionRequest,
  responseType: proto_api_v1_robot_pb.ExecuteFunctionResponse
};

RobotService.ExecuteSource = {
  methodName: "ExecuteSource",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ExecuteSourceRequest,
  responseType: proto_api_v1_robot_pb.ExecuteSourceResponse
};

RobotService.ResourceRunCommand = {
  methodName: "ResourceRunCommand",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ResourceRunCommandRequest,
  responseType: proto_api_v1_robot_pb.ResourceRunCommandResponse
};

RobotService.FrameServiceConfig = {
  methodName: "FrameServiceConfig",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.FrameServiceConfigRequest,
  responseType: proto_api_v1_robot_pb.FrameServiceConfigResponse
};

RobotService.NavigationServiceMode = {
  methodName: "NavigationServiceMode",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.NavigationServiceModeRequest,
  responseType: proto_api_v1_robot_pb.NavigationServiceModeResponse
};

RobotService.NavigationServiceSetMode = {
  methodName: "NavigationServiceSetMode",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.NavigationServiceSetModeRequest,
  responseType: proto_api_v1_robot_pb.NavigationServiceSetModeResponse
};

RobotService.NavigationServiceLocation = {
  methodName: "NavigationServiceLocation",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.NavigationServiceLocationRequest,
  responseType: proto_api_v1_robot_pb.NavigationServiceLocationResponse
};

RobotService.NavigationServiceWaypoints = {
  methodName: "NavigationServiceWaypoints",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.NavigationServiceWaypointsRequest,
  responseType: proto_api_v1_robot_pb.NavigationServiceWaypointsResponse
};

RobotService.NavigationServiceAddWaypoint = {
  methodName: "NavigationServiceAddWaypoint",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.NavigationServiceAddWaypointRequest,
  responseType: proto_api_v1_robot_pb.NavigationServiceAddWaypointResponse
};

RobotService.NavigationServiceRemoveWaypoint = {
  methodName: "NavigationServiceRemoveWaypoint",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.NavigationServiceRemoveWaypointRequest,
  responseType: proto_api_v1_robot_pb.NavigationServiceRemoveWaypointResponse
};

RobotService.ObjectManipulationServiceDoGrab = {
  methodName: "ObjectManipulationServiceDoGrab",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabRequest,
  responseType: proto_api_v1_robot_pb.ObjectManipulationServiceDoGrabResponse
};

exports.RobotService = RobotService;

function RobotServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

RobotServiceClient.prototype.status = function status(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.Status, {
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

RobotServiceClient.prototype.statusStream = function statusStream(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(RobotService.StatusStream, {
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

RobotServiceClient.prototype.config = function config(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.Config, {
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

RobotServiceClient.prototype.doAction = function doAction(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.DoAction, {
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

RobotServiceClient.prototype.sensorReadings = function sensorReadings(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.SensorReadings, {
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

RobotServiceClient.prototype.executeFunction = function executeFunction(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ExecuteFunction, {
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

RobotServiceClient.prototype.executeSource = function executeSource(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ExecuteSource, {
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

RobotServiceClient.prototype.resourceRunCommand = function resourceRunCommand(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ResourceRunCommand, {
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

RobotServiceClient.prototype.frameServiceConfig = function frameServiceConfig(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.FrameServiceConfig, {
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

RobotServiceClient.prototype.navigationServiceMode = function navigationServiceMode(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.NavigationServiceMode, {
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

RobotServiceClient.prototype.navigationServiceSetMode = function navigationServiceSetMode(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.NavigationServiceSetMode, {
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

RobotServiceClient.prototype.navigationServiceLocation = function navigationServiceLocation(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.NavigationServiceLocation, {
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

RobotServiceClient.prototype.navigationServiceWaypoints = function navigationServiceWaypoints(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.NavigationServiceWaypoints, {
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

RobotServiceClient.prototype.navigationServiceAddWaypoint = function navigationServiceAddWaypoint(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.NavigationServiceAddWaypoint, {
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

RobotServiceClient.prototype.navigationServiceRemoveWaypoint = function navigationServiceRemoveWaypoint(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.NavigationServiceRemoveWaypoint, {
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

RobotServiceClient.prototype.objectManipulationServiceDoGrab = function objectManipulationServiceDoGrab(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ObjectManipulationServiceDoGrab, {
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

exports.RobotServiceClient = RobotServiceClient;

