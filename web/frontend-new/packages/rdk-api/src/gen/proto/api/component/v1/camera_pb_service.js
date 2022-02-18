// package: proto.api.component.v1
// file: proto/api/component/v1/camera.proto

var proto_api_component_v1_camera_pb = require("../../../../proto/api/component/v1/camera_pb");
var google_api_httpbody_pb = require("../../../../google/api/httpbody_pb");
var grpc = require("@improbable-eng/grpc-web").grpc;

var CameraService = (function () {
  function CameraService() {}
  CameraService.serviceName = "proto.api.component.v1.CameraService";
  return CameraService;
}());

CameraService.GetFrame = {
  methodName: "GetFrame",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServiceGetFrameRequest,
  responseType: proto_api_component_v1_camera_pb.CameraServiceGetFrameResponse
};

CameraService.RenderFrame = {
  methodName: "RenderFrame",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServiceRenderFrameRequest,
  responseType: google_api_httpbody_pb.HttpBody
};

CameraService.GetPointCloud = {
  methodName: "GetPointCloud",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServiceGetPointCloudRequest,
  responseType: proto_api_component_v1_camera_pb.CameraServiceGetPointCloudResponse
};

CameraService.GetObjectPointClouds = {
  methodName: "GetObjectPointClouds",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServiceGetObjectPointCloudsRequest,
  responseType: proto_api_component_v1_camera_pb.CameraServiceGetObjectPointCloudsResponse
};

exports.CameraService = CameraService;

function CameraServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

CameraServiceClient.prototype.getFrame = function getFrame(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(CameraService.GetFrame, {
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

CameraServiceClient.prototype.renderFrame = function renderFrame(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(CameraService.RenderFrame, {
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

CameraServiceClient.prototype.getPointCloud = function getPointCloud(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(CameraService.GetPointCloud, {
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

CameraServiceClient.prototype.getObjectPointClouds = function getObjectPointClouds(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(CameraService.GetObjectPointClouds, {
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

exports.CameraServiceClient = CameraServiceClient;

