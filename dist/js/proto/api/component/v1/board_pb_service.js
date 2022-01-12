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

BoardService.GPIOSet = {
  methodName: "GPIOSet",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceGPIOSetRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceGPIOSetResponse
};

BoardService.GPIOGet = {
  methodName: "GPIOGet",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceGPIOGetRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceGPIOGetResponse
};

BoardService.PWMSet = {
  methodName: "PWMSet",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServicePWMSetRequest,
  responseType: proto_api_component_v1_board_pb.BoardServicePWMSetResponse
};

BoardService.PWMSetFrequency = {
  methodName: "PWMSetFrequency",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyRequest,
  responseType: proto_api_component_v1_board_pb.BoardServicePWMSetFrequencyResponse
};

BoardService.AnalogReaderRead = {
  methodName: "AnalogReaderRead",
  service: BoardService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadRequest,
  responseType: proto_api_component_v1_board_pb.BoardServiceAnalogReaderReadResponse
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

BoardServiceClient.prototype.gPIOSet = function gPIOSet(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.GPIOSet, {
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

BoardServiceClient.prototype.gPIOGet = function gPIOGet(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.GPIOGet, {
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

BoardServiceClient.prototype.pWMSet = function pWMSet(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.PWMSet, {
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

BoardServiceClient.prototype.pWMSetFrequency = function pWMSetFrequency(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.PWMSetFrequency, {
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

BoardServiceClient.prototype.analogReaderRead = function analogReaderRead(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BoardService.AnalogReaderRead, {
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

