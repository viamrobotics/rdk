// package: proto.api.component.v1
// file: proto/api/component/v1/base.proto

var proto_api_component_v1_base_pb = require("../../../../proto/api/component/v1/base_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var BaseService = (function () {
  function BaseService() {}
  BaseService.serviceName = "proto.api.component.v1.BaseService";
  return BaseService;
}());

BaseService.MoveStraight = {
  methodName: "MoveStraight",
  service: BaseService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_base_pb.BaseServiceMoveStraightRequest,
  responseType: proto_api_component_v1_base_pb.BaseServiceMoveStraightResponse
};

BaseService.MoveArc = {
  methodName: "MoveArc",
  service: BaseService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_base_pb.BaseServiceMoveArcRequest,
  responseType: proto_api_component_v1_base_pb.BaseServiceMoveArcResponse
};

BaseService.Spin = {
  methodName: "Spin",
  service: BaseService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_base_pb.BaseServiceSpinRequest,
  responseType: proto_api_component_v1_base_pb.BaseServiceSpinResponse
};

BaseService.Stop = {
  methodName: "Stop",
  service: BaseService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_base_pb.BaseServiceStopRequest,
  responseType: proto_api_component_v1_base_pb.BaseServiceStopResponse
};

exports.BaseService = BaseService;

function BaseServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

BaseServiceClient.prototype.moveStraight = function moveStraight(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BaseService.MoveStraight, {
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

BaseServiceClient.prototype.moveArc = function moveArc(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BaseService.MoveArc, {
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

BaseServiceClient.prototype.spin = function spin(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BaseService.Spin, {
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

BaseServiceClient.prototype.stop = function stop(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(BaseService.Stop, {
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

exports.BaseServiceClient = BaseServiceClient;

