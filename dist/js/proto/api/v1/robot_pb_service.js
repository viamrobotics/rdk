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

RobotService.ForceMatrixMatrix = {
  methodName: "ForceMatrixMatrix",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ForceMatrixMatrixRequest,
  responseType: proto_api_v1_robot_pb.ForceMatrixMatrixResponse
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

RobotService.ServoMove = {
  methodName: "ServoMove",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ServoMoveRequest,
  responseType: proto_api_v1_robot_pb.ServoMoveResponse
};

RobotService.ServoCurrent = {
  methodName: "ServoCurrent",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ServoCurrentRequest,
  responseType: proto_api_v1_robot_pb.ServoCurrentResponse
};

RobotService.MotorPower = {
  methodName: "MotorPower",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorPowerRequest,
  responseType: proto_api_v1_robot_pb.MotorPowerResponse
};

RobotService.MotorGo = {
  methodName: "MotorGo",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorGoRequest,
  responseType: proto_api_v1_robot_pb.MotorGoResponse
};

RobotService.MotorGoFor = {
  methodName: "MotorGoFor",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorGoForRequest,
  responseType: proto_api_v1_robot_pb.MotorGoForResponse
};

RobotService.MotorGoTo = {
  methodName: "MotorGoTo",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorGoToRequest,
  responseType: proto_api_v1_robot_pb.MotorGoToResponse
};

RobotService.MotorGoTillStop = {
  methodName: "MotorGoTillStop",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorGoTillStopRequest,
  responseType: proto_api_v1_robot_pb.MotorGoTillStopResponse
};

RobotService.MotorZero = {
  methodName: "MotorZero",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorZeroRequest,
  responseType: proto_api_v1_robot_pb.MotorZeroResponse
};

RobotService.MotorPosition = {
  methodName: "MotorPosition",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorPositionRequest,
  responseType: proto_api_v1_robot_pb.MotorPositionResponse
};

RobotService.MotorPositionSupported = {
  methodName: "MotorPositionSupported",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorPositionSupportedRequest,
  responseType: proto_api_v1_robot_pb.MotorPositionSupportedResponse
};

RobotService.MotorOff = {
  methodName: "MotorOff",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorOffRequest,
  responseType: proto_api_v1_robot_pb.MotorOffResponse
};

RobotService.MotorIsOn = {
  methodName: "MotorIsOn",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.MotorIsOnRequest,
  responseType: proto_api_v1_robot_pb.MotorIsOnResponse
};

RobotService.InputControllerControls = {
  methodName: "InputControllerControls",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.InputControllerControlsRequest,
  responseType: proto_api_v1_robot_pb.InputControllerControlsResponse
};

RobotService.InputControllerLastEvents = {
  methodName: "InputControllerLastEvents",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.InputControllerLastEventsRequest,
  responseType: proto_api_v1_robot_pb.InputControllerLastEventsResponse
};

RobotService.InputControllerEventStream = {
  methodName: "InputControllerEventStream",
  service: RobotService,
  requestStream: false,
  responseStream: true,
  requestType: proto_api_v1_robot_pb.InputControllerEventStreamRequest,
  responseType: proto_api_v1_robot_pb.InputControllerEvent
};

RobotService.ResourceRunCommand = {
  methodName: "ResourceRunCommand",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.ResourceRunCommandRequest,
  responseType: proto_api_v1_robot_pb.ResourceRunCommandResponse
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

RobotService.IMUAngularVelocity = {
  methodName: "IMUAngularVelocity",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.IMUAngularVelocityRequest,
  responseType: proto_api_v1_robot_pb.IMUAngularVelocityResponse
};

RobotService.IMUOrientation = {
  methodName: "IMUOrientation",
  service: RobotService,
  requestStream: false,
  responseStream: false,
  requestType: proto_api_v1_robot_pb.IMUOrientationRequest,
  responseType: proto_api_v1_robot_pb.IMUOrientationResponse
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

RobotServiceClient.prototype.forceMatrixMatrix = function forceMatrixMatrix(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ForceMatrixMatrix, {
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

RobotServiceClient.prototype.servoMove = function servoMove(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ServoMove, {
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

RobotServiceClient.prototype.servoCurrent = function servoCurrent(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.ServoCurrent, {
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

RobotServiceClient.prototype.motorPower = function motorPower(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorPower, {
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

RobotServiceClient.prototype.motorGo = function motorGo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorGo, {
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

RobotServiceClient.prototype.motorGoFor = function motorGoFor(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorGoFor, {
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

RobotServiceClient.prototype.motorGoTo = function motorGoTo(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorGoTo, {
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

RobotServiceClient.prototype.motorGoTillStop = function motorGoTillStop(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorGoTillStop, {
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

RobotServiceClient.prototype.motorZero = function motorZero(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorZero, {
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

RobotServiceClient.prototype.motorPosition = function motorPosition(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorPosition, {
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

RobotServiceClient.prototype.motorPositionSupported = function motorPositionSupported(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorPositionSupported, {
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

RobotServiceClient.prototype.motorOff = function motorOff(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorOff, {
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

RobotServiceClient.prototype.motorIsOn = function motorIsOn(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.MotorIsOn, {
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

RobotServiceClient.prototype.inputControllerControls = function inputControllerControls(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.InputControllerControls, {
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

RobotServiceClient.prototype.inputControllerLastEvents = function inputControllerLastEvents(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.InputControllerLastEvents, {
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

RobotServiceClient.prototype.inputControllerEventStream = function inputControllerEventStream(requestMessage, metadata) {
  var listeners = {
    data: [],
    end: [],
    status: []
  };
  var client = grpc.invoke(RobotService.InputControllerEventStream, {
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

RobotServiceClient.prototype.iMUAngularVelocity = function iMUAngularVelocity(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.IMUAngularVelocity, {
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

RobotServiceClient.prototype.iMUOrientation = function iMUOrientation(requestMessage, metadata, callback) {
  if (arguments.length === 2) {
    callback = arguments[1];
  }
  var client = grpc.unary(RobotService.IMUOrientation, {
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

