// package: proto.api.component.v1
// file: proto/api/component/v1/input_controller.proto

var proto_api_component_v1_input_controller_pb = require("../../../../proto/api/component/v1/input_controller_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var InputControllerService = (function () {
  function InputControllerService() {}
  InputControllerService.serviceName = "proto.api.component.v1.InputControllerService";
  return InputControllerService;
}());

InputControllerService.GetControls = {
  methodName: "GetControls",
  service: InputControllerService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceGetControlsResponse
};

InputControllerService.GetEvents = {
  methodName: "GetEvents",
  service: InputControllerService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceGetEventsResponse
};

InputControllerService.StreamEvents = {
  methodName: "StreamEvents",
  service: InputControllerService,
  requestStream: false,
  responseStream: true,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceStreamEventsRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceStreamEventsResponse
};

InputControllerService.TriggerEvent = {
  methodName: "TriggerEvent",
  service: InputControllerService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceTriggerEventResponse
};

exports.InputControllerService = InputControllerService;

function InputControllerServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

InputControllerServiceClient.prototype.getControls = function getControls(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(InputControllerService.GetControls, {
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

InputControllerServiceClient.prototype.getEvents = function getEvents(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(InputControllerService.GetEvents, {
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

InputControllerServiceClient.prototype.streamEvents = function streamEvents(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(InputControllerService.StreamEvents, {
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

InputControllerServiceClient.prototype.triggerEvent = function triggerEvent(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(InputControllerService.TriggerEvent, {
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

exports.InputControllerServiceClient = InputControllerServiceClient;

