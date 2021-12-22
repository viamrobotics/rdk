// package: proto.stream.v1
// file: proto/stream/v1/stream.proto

var proto_stream_v1_stream_pb = require("../../../proto/stream/v1/stream_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var StreamService = (function () {
  function StreamService() {}
  StreamService.serviceName = "proto.stream.v1.StreamService";
  return StreamService;
}());

StreamService.ListStreams = {
  methodName: "ListStreams",
  service: StreamService,
  requestStream: false,
  responseStream: false,
  requestType: proto_stream_v1_stream_pb.ListStreamsRequest,
  responseType: proto_stream_v1_stream_pb.ListStreamsResponse
};

StreamService.AddStream = {
  methodName: "AddStream",
  service: StreamService,
  requestStream: false,
  responseStream: false,
  requestType: proto_stream_v1_stream_pb.AddStreamRequest,
  responseType: proto_stream_v1_stream_pb.AddStreamResponse
};

exports.StreamService = StreamService;

function StreamServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

StreamServiceClient.prototype.listStreams = function listStreams(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(StreamService.ListStreams, {
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

StreamServiceClient.prototype.addStream = function addStream(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(StreamService.AddStream, {
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

exports.StreamServiceClient = StreamServiceClient;

