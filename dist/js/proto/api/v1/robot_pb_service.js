// package: proto.api.v1
// file: proto/api/v1/robot.proto

var proto_api_v1_robot_pb = require("../../../proto/api/v1/robot_pb");
var google_api_httpbody_pb = require("../../../google/api/httpbody_pb");
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

RobotService.ArmCurrentPosition = {
  methodName: "ArmCurrentPosition",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ArmCurrentPositionRequest,
  responseType: proto_api_v1_robot_pb.ArmCurrentPositionResponse
};

RobotService.ArmMoveToPosition = {
  methodName: "ArmMoveToPosition",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ArmMoveToPositionRequest,
  responseType: proto_api_v1_robot_pb.ArmMoveToPositionResponse
};

RobotService.ArmCurrentJointPositions = {
  methodName: "ArmCurrentJointPositions",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ArmCurrentJointPositionsRequest,
  responseType: proto_api_v1_robot_pb.ArmCurrentJointPositionsResponse
};

RobotService.ArmMoveToJointPositions = {
  methodName: "ArmMoveToJointPositions",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ArmMoveToJointPositionsRequest,
  responseType: proto_api_v1_robot_pb.ArmMoveToJointPositionsResponse
};

RobotService.ArmJointMoveDelta = {
  methodName: "ArmJointMoveDelta",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ArmJointMoveDeltaRequest,
  responseType: proto_api_v1_robot_pb.ArmJointMoveDeltaResponse
};

RobotService.BaseMoveStraight = {
  methodName: "BaseMoveStraight",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BaseMoveStraightRequest,
  responseType: proto_api_v1_robot_pb.BaseMoveStraightResponse
};

RobotService.BaseSpin = {
  methodName: "BaseSpin",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BaseSpinRequest,
  responseType: proto_api_v1_robot_pb.BaseSpinResponse
};

RobotService.BaseStop = {
  methodName: "BaseStop",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BaseStopRequest,
  responseType: proto_api_v1_robot_pb.BaseStopResponse
};

RobotService.BaseWidthMillis = {
  methodName: "BaseWidthMillis",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BaseWidthMillisRequest,
  responseType: proto_api_v1_robot_pb.BaseWidthMillisResponse
};

RobotService.GripperOpen = {
  methodName: "GripperOpen",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.GripperOpenRequest,
  responseType: proto_api_v1_robot_pb.GripperOpenResponse
};

RobotService.GripperGrab = {
  methodName: "GripperGrab",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.GripperGrabRequest,
  responseType: proto_api_v1_robot_pb.GripperGrabResponse
};

RobotService.CameraFrame = {
  methodName: "CameraFrame",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.CameraFrameRequest,
  responseType: proto_api_v1_robot_pb.CameraFrameResponse
};

RobotService.CameraRenderFrame = {
  methodName: "CameraRenderFrame",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.CameraRenderFrameRequest,
  responseType: google_api_httpbody_pb.HttpBody
};

RobotService.PointCloud = {
  methodName: "PointCloud",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.PointCloudRequest,
  responseType: proto_api_v1_robot_pb.PointCloudResponse
};

RobotService.ObjectPointClouds = {
  methodName: "ObjectPointClouds",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ObjectPointCloudsRequest,
  responseType: proto_api_v1_robot_pb.ObjectPointCloudsResponse
};

RobotService.LidarInfo = {
  methodName: "LidarInfo",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.LidarInfoRequest,
  responseType: proto_api_v1_robot_pb.LidarInfoResponse
};

RobotService.LidarStart = {
  methodName: "LidarStart",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.LidarStartRequest,
  responseType: proto_api_v1_robot_pb.LidarStartResponse
};

RobotService.LidarStop = {
  methodName: "LidarStop",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.LidarStopRequest,
  responseType: proto_api_v1_robot_pb.LidarStopResponse
};

RobotService.LidarScan = {
  methodName: "LidarScan",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.LidarScanRequest,
  responseType: proto_api_v1_robot_pb.LidarScanResponse
};

RobotService.LidarRange = {
  methodName: "LidarRange",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.LidarRangeRequest,
  responseType: proto_api_v1_robot_pb.LidarRangeResponse
};

RobotService.LidarBounds = {
  methodName: "LidarBounds",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.LidarBoundsRequest,
  responseType: proto_api_v1_robot_pb.LidarBoundsResponse
};

RobotService.LidarAngularResolution = {
  methodName: "LidarAngularResolution",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.LidarAngularResolutionRequest,
  responseType: proto_api_v1_robot_pb.LidarAngularResolutionResponse
};

RobotService.BoardStatus = {
  methodName: "BoardStatus",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardStatusRequest,
  responseType: proto_api_v1_robot_pb.BoardStatusResponse
};

RobotService.BoardGPIOSet = {
  methodName: "BoardGPIOSet",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardGPIOSetRequest,
  responseType: proto_api_v1_robot_pb.BoardGPIOSetResponse
};

RobotService.BoardGPIOGet = {
  methodName: "BoardGPIOGet",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardGPIOGetRequest,
  responseType: proto_api_v1_robot_pb.BoardGPIOGetResponse
};

RobotService.BoardPWMSet = {
  methodName: "BoardPWMSet",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardPWMSetRequest,
  responseType: proto_api_v1_robot_pb.BoardPWMSetResponse
};

RobotService.BoardPWMSetFrequency = {
  methodName: "BoardPWMSetFrequency",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardPWMSetFrequencyRequest,
  responseType: proto_api_v1_robot_pb.BoardPWMSetFrequencyResponse
};

RobotService.BoardMotorPower = {
  methodName: "BoardMotorPower",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorPowerRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorPowerResponse
};

RobotService.BoardMotorGo = {
  methodName: "BoardMotorGo",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorGoRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorGoResponse
};

RobotService.BoardMotorGoFor = {
  methodName: "BoardMotorGoFor",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorGoForRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorGoForResponse
};

RobotService.BoardMotorGoTo = {
  methodName: "BoardMotorGoTo",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorGoToRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorGoToResponse
};

RobotService.BoardMotorGoTillStop = {
  methodName: "BoardMotorGoTillStop",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorGoTillStopRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorGoTillStopResponse
};

RobotService.BoardMotorZero = {
  methodName: "BoardMotorZero",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorZeroRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorZeroResponse
};

RobotService.BoardMotorPosition = {
  methodName: "BoardMotorPosition",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorPositionRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorPositionResponse
};

RobotService.BoardMotorPositionSupported = {
  methodName: "BoardMotorPositionSupported",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorPositionSupportedRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorPositionSupportedResponse
};

RobotService.BoardMotorOff = {
  methodName: "BoardMotorOff",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorOffRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorOffResponse
};

RobotService.BoardMotorIsOn = {
  methodName: "BoardMotorIsOn",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardMotorIsOnRequest,
  responseType: proto_api_v1_robot_pb.BoardMotorIsOnResponse
};

RobotService.BoardServoMove = {
  methodName: "BoardServoMove",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardServoMoveRequest,
  responseType: proto_api_v1_robot_pb.BoardServoMoveResponse
};

RobotService.BoardServoCurrent = {
  methodName: "BoardServoCurrent",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardServoCurrentRequest,
  responseType: proto_api_v1_robot_pb.BoardServoCurrentResponse
};

RobotService.BoardAnalogReaderRead = {
  methodName: "BoardAnalogReaderRead",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardAnalogReaderReadRequest,
  responseType: proto_api_v1_robot_pb.BoardAnalogReaderReadResponse
};

RobotService.BoardDigitalInterruptConfig = {
  methodName: "BoardDigitalInterruptConfig",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardDigitalInterruptConfigRequest,
  responseType: proto_api_v1_robot_pb.BoardDigitalInterruptConfigResponse
};

RobotService.BoardDigitalInterruptValue = {
  methodName: "BoardDigitalInterruptValue",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardDigitalInterruptValueRequest,
  responseType: proto_api_v1_robot_pb.BoardDigitalInterruptValueResponse
};

RobotService.BoardDigitalInterruptTick = {
  methodName: "BoardDigitalInterruptTick",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.BoardDigitalInterruptTickRequest,
  responseType: proto_api_v1_robot_pb.BoardDigitalInterruptTickResponse
};

RobotService.SensorReadings = {
  methodName: "SensorReadings",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.SensorReadingsRequest,
  responseType: proto_api_v1_robot_pb.SensorReadingsResponse
};

RobotService.CompassHeading = {
  methodName: "CompassHeading",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.CompassHeadingRequest,
  responseType: proto_api_v1_robot_pb.CompassHeadingResponse
};

RobotService.CompassStartCalibration = {
  methodName: "CompassStartCalibration",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.CompassStartCalibrationRequest,
  responseType: proto_api_v1_robot_pb.CompassStartCalibrationResponse
};

RobotService.CompassStopCalibration = {
  methodName: "CompassStopCalibration",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.CompassStopCalibrationRequest,
  responseType: proto_api_v1_robot_pb.CompassStopCalibrationResponse
};

RobotService.CompassMark = {
  methodName: "CompassMark",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.CompassMarkRequest,
  responseType: proto_api_v1_robot_pb.CompassMarkResponse
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

RobotServiceClient.prototype.armCurrentPosition = function armCurrentPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ArmCurrentPosition, {
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

RobotServiceClient.prototype.armMoveToPosition = function armMoveToPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ArmMoveToPosition, {
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

RobotServiceClient.prototype.armCurrentJointPositions = function armCurrentJointPositions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ArmCurrentJointPositions, {
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

RobotServiceClient.prototype.armMoveToJointPositions = function armMoveToJointPositions(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ArmMoveToJointPositions, {
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

RobotServiceClient.prototype.armJointMoveDelta = function armJointMoveDelta(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ArmJointMoveDelta, {
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

RobotServiceClient.prototype.baseMoveStraight = function baseMoveStraight(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BaseMoveStraight, {
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

RobotServiceClient.prototype.baseSpin = function baseSpin(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BaseSpin, {
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

RobotServiceClient.prototype.baseStop = function baseStop(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BaseStop, {
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

RobotServiceClient.prototype.baseWidthMillis = function baseWidthMillis(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BaseWidthMillis, {
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

RobotServiceClient.prototype.gripperOpen = function gripperOpen(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.GripperOpen, {
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

RobotServiceClient.prototype.gripperGrab = function gripperGrab(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.GripperGrab, {
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

RobotServiceClient.prototype.cameraFrame = function cameraFrame(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.CameraFrame, {
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

RobotServiceClient.prototype.cameraRenderFrame = function cameraRenderFrame(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.CameraRenderFrame, {
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

RobotServiceClient.prototype.pointCloud = function pointCloud(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.PointCloud, {
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

RobotServiceClient.prototype.objectPointClouds = function objectPointClouds(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ObjectPointClouds, {
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

RobotServiceClient.prototype.lidarInfo = function lidarInfo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.LidarInfo, {
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

RobotServiceClient.prototype.lidarStart = function lidarStart(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.LidarStart, {
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

RobotServiceClient.prototype.lidarStop = function lidarStop(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.LidarStop, {
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

RobotServiceClient.prototype.lidarScan = function lidarScan(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.LidarScan, {
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

RobotServiceClient.prototype.lidarRange = function lidarRange(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.LidarRange, {
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

RobotServiceClient.prototype.lidarBounds = function lidarBounds(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.LidarBounds, {
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

RobotServiceClient.prototype.lidarAngularResolution = function lidarAngularResolution(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.LidarAngularResolution, {
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

RobotServiceClient.prototype.boardStatus = function boardStatus(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardStatus, {
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

RobotServiceClient.prototype.boardGPIOSet = function boardGPIOSet(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardGPIOSet, {
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

RobotServiceClient.prototype.boardGPIOGet = function boardGPIOGet(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardGPIOGet, {
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

RobotServiceClient.prototype.boardPWMSet = function boardPWMSet(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardPWMSet, {
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

RobotServiceClient.prototype.boardPWMSetFrequency = function boardPWMSetFrequency(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardPWMSetFrequency, {
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

RobotServiceClient.prototype.boardMotorPower = function boardMotorPower(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorPower, {
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

RobotServiceClient.prototype.boardMotorGo = function boardMotorGo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorGo, {
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

RobotServiceClient.prototype.boardMotorGoFor = function boardMotorGoFor(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorGoFor, {
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

RobotServiceClient.prototype.boardMotorGoTo = function boardMotorGoTo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorGoTo, {
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

RobotServiceClient.prototype.boardMotorGoTillStop = function boardMotorGoTillStop(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorGoTillStop, {
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

RobotServiceClient.prototype.boardMotorZero = function boardMotorZero(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorZero, {
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

RobotServiceClient.prototype.boardMotorPosition = function boardMotorPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorPosition, {
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

RobotServiceClient.prototype.boardMotorPositionSupported = function boardMotorPositionSupported(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorPositionSupported, {
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

RobotServiceClient.prototype.boardMotorOff = function boardMotorOff(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorOff, {
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

RobotServiceClient.prototype.boardMotorIsOn = function boardMotorIsOn(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardMotorIsOn, {
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

RobotServiceClient.prototype.boardServoMove = function boardServoMove(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardServoMove, {
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

RobotServiceClient.prototype.boardServoCurrent = function boardServoCurrent(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardServoCurrent, {
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

RobotServiceClient.prototype.boardAnalogReaderRead = function boardAnalogReaderRead(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardAnalogReaderRead, {
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

RobotServiceClient.prototype.boardDigitalInterruptConfig = function boardDigitalInterruptConfig(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardDigitalInterruptConfig, {
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

RobotServiceClient.prototype.boardDigitalInterruptValue = function boardDigitalInterruptValue(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardDigitalInterruptValue, {
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

RobotServiceClient.prototype.boardDigitalInterruptTick = function boardDigitalInterruptTick(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.BoardDigitalInterruptTick, {
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

RobotServiceClient.prototype.compassHeading = function compassHeading(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.CompassHeading, {
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

RobotServiceClient.prototype.compassStartCalibration = function compassStartCalibration(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.CompassStartCalibration, {
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

RobotServiceClient.prototype.compassStopCalibration = function compassStopCalibration(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.CompassStopCalibration, {
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

RobotServiceClient.prototype.compassMark = function compassMark(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.CompassMark, {
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

exports.RobotServiceClient = RobotServiceClient;

