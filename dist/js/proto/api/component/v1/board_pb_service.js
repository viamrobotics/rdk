// package: proto.api.component.v1
// file: proto/api/component/v1/board.proto

var proto_api_component_v1_board_pb = require("../../../../proto/api/component/v1/board_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var BoardService = (function () {
  function BoardService() {}
  BoardService.serviceName = "proto.api.component.v1.BoardService";
  return BoardService;
}());

BoardService.Status = {
  methodName: "Status",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceStatusRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceStatusResponse
};

BoardService.SetGPIO = {
  methodName: "SetGPIO",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceSetGPIORequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceSetGPIOResponse
};

BoardService.GetGPIO = {
  methodName: "GetGPIO",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceGetGPIORequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceGetGPIOResponse
};

BoardService.SetPWM = {
  methodName: "SetPWM",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceSetPWMRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceSetPWMResponse
};

BoardService.SetPWMFrequency = {
  methodName: "SetPWMFrequency",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceSetPWMFrequencyResponse
};

BoardService.ReadAnalogReader = {
  methodName: "ReadAnalogReader",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceReadAnalogReaderResponse
};

BoardService.DigitalInterruptConfig = {
  methodName: "DigitalInterruptConfig",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptConfigResponse
};

BoardService.DigitalInterruptValue = {
  methodName: "DigitalInterruptValue",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptValueResponse
};

BoardService.DigitalInterruptTick = {
  methodName: "DigitalInterruptTick",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceDigitalInterruptTickResponse
};

exports.BoardService = BoardService;

function BoardServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

BoardServiceClient.prototype.status = function status(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.Status, {
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

BoardServiceClient.prototype.setGPIO = function setGPIO(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.SetGPIO, {
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

BoardServiceClient.prototype.getGPIO = function getGPIO(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.GetGPIO, {
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

BoardServiceClient.prototype.setPWM = function setPWM(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.SetPWM, {
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

BoardServiceClient.prototype.setPWMFrequency = function setPWMFrequency(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.SetPWMFrequency, {
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

BoardServiceClient.prototype.readAnalogReader = function readAnalogReader(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.ReadAnalogReader, {
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

BoardServiceClient.prototype.digitalInterruptConfig = function digitalInterruptConfig(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.DigitalInterruptConfig, {
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

BoardServiceClient.prototype.digitalInterruptValue = function digitalInterruptValue(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.DigitalInterruptValue, {
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

BoardServiceClient.prototype.digitalInterruptTick = function digitalInterruptTick(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.DigitalInterruptTick, {
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

exports.BoardServiceClient = BoardServiceClient;

