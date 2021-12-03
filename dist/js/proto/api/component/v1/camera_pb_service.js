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

CameraService.Frame = {
  methodName: "Frame",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServiceFrameRequest,
  responseType: proto_api_component_v1_camera_pb.CameraServiceFrameResponse
};

CameraService.RenderFrame = {
  methodName: "RenderFrame",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServiceRenderFrameRequest,
  responseType: google_api_httpbody_pb.HttpBody
};

CameraService.PointCloud = {
  methodName: "PointCloud",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServicePointCloudRequest,
  responseType: proto_api_component_v1_camera_pb.CameraServicePointCloudResponse
};

CameraService.ObjectPointClouds = {
  methodName: "ObjectPointClouds",
  service: CameraService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsRequest,
  responseType: proto_api_component_v1_camera_pb.CameraServiceObjectPointCloudsResponse
};

exports.CameraService = CameraService;

function CameraServiceClient(serviceHost, options) {
  this.serviceHost = serviceHost;
  this.options = options || {};
}

CameraServiceClient.prototype.frame = function frame(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(CameraService.Frame, {
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

CameraServiceClient.prototype.pointCloud = function pointCloud(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(CameraService.PointCloud, {
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

CameraServiceClient.prototype.objectPointClouds = function objectPointClouds(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(CameraService.ObjectPointClouds, {
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

