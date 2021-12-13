// package: proto.api.component.v1
// file: proto/api/component/v1/input_controller.proto

var proto_api_component_v1_input_controller_pb = require("../../../../proto/api/component/v1/input_controller_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var InputControllerService = (function () {
  function InputControllerService() {}
  InputControllerService.serviceName = "proto.api.component.v1.InputControllerService";
  return InputControllerService;
}());

InputControllerService.Controls = {
  methodName: "Controls",
  service: InputControllerService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceControlsRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceControlsResponse
};

InputControllerService.LastEvents = {
  methodName: "LastEvents",
  service: InputControllerService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceLastEventsResponse
};

InputControllerService.EventStream = {
  methodName: "EventStream",
  service: InputControllerService,
  requestStream: false,
  responseStream: true,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceEventStreamRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceEventStreamResponse
};

InputControllerService.InjectEvent = {
  methodName: "InjectEvent",
  service: InputControllerService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventRequest,
  responseType: proto_api_component_v1_input_controller_pb.InputControllerServiceInjectEventResponse
};

exports.InputControllerService = InputControllerService;

function InputControllerServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

InputControllerServiceClient.prototype.controls = function controls(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(InputControllerService.Controls, {
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

InputControllerServiceClient.prototype.lastEvents = function lastEvents(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(InputControllerService.LastEvents, {
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

InputControllerServiceClient.prototype.eventStream = function eventStream(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(InputControllerService.EventStream, {
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

InputControllerServiceClient.prototype.injectEvent = function injectEvent(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(InputControllerService.InjectEvent, {
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

