// source: proto/api/v1/robot.proto
/**
 * @fileoverview
 * @enhanceable
 * @suppress {missingRequire} reports error on implicit type usages.
 * @suppress {messageConventions} JS Compiler reports an error if a variable or
 *     field starts with 'MSG_' and isn't a translatable message.
 * @public
 */
// GENERATED CODE -- DO NOT EDIT!
/* eslint-disable */
// @ts-nocheck

var jspb = require('google-protobuf');
var goog = jspb;
var global = (function() {
  if (this) { return this; }
  if (typeof window !== 'undefined') { return window; }
  if (typeof global !== 'undefined') { return global; }
  if (typeof self !== 'undefined') { return self; }
  return Function('return this')();
}.call(null));

var google_protobuf_struct_pb = require('google-protobuf/google/protobuf/struct_pb.js');
goog.object.extend(proto, google_protobuf_struct_pb);
var google_protobuf_duration_pb = require('google-protobuf/google/protobuf/duration_pb.js');
goog.object.extend(proto, google_protobuf_duration_pb);
var google_protobuf_timestamp_pb = require('google-protobuf/google/protobuf/timestamp_pb.js');
goog.object.extend(proto, google_protobuf_timestamp_pb);
var google_api_annotations_pb = require('../../../google/api/annotations_pb.js');
goog.object.extend(proto, google_api_annotations_pb);
var proto_api_common_v1_common_pb = require('../../../proto/api/common/v1/common_pb.js');
goog.object.extend(proto, proto_api_common_v1_common_pb);
goog.exportSymbol('proto.proto.api.v1.ArmStatus', null, global);
goog.exportSymbol('proto.proto.api.v1.ComponentConfig', null, global);
goog.exportSymbol('proto.proto.api.v1.ConfigRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.ConfigResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.DoActionRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.DoActionResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.ExecuteFunctionRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.ExecuteFunctionResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.ExecuteSourceRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.ExecuteSourceResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.FrameConfig', null, global);
goog.exportSymbol('proto.proto.api.v1.FrameServiceConfigRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.FrameServiceConfigResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.FrameSystemConfig', null, global);
goog.exportSymbol('proto.proto.api.v1.GantryStatus', null, global);
goog.exportSymbol('proto.proto.api.v1.InputControllerEvent', null, global);
goog.exportSymbol('proto.proto.api.v1.InputControllerStatus', null, global);
goog.exportSymbol('proto.proto.api.v1.JointPositions', null, global);
goog.exportSymbol('proto.proto.api.v1.MotorStatus', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceAddWaypointRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceAddWaypointResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceLocationRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceLocationResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceMode', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceModeRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceModeResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceRemoveWaypointRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceRemoveWaypointResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceSetModeRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceSetModeResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceWaypoint', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceWaypointsRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.NavigationServiceWaypointsResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.Pose', null, global);
goog.exportSymbol('proto.proto.api.v1.ResourceRunCommandRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.ResourceRunCommandResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.SensorReadingsRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.SensorReadingsResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.SensorStatus', null, global);
goog.exportSymbol('proto.proto.api.v1.ServoStatus', null, global);
goog.exportSymbol('proto.proto.api.v1.Status', null, global);
goog.exportSymbol('proto.proto.api.v1.StatusRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.StatusResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.StatusStreamRequest', null, global);
goog.exportSymbol('proto.proto.api.v1.StatusStreamResponse', null, global);
goog.exportSymbol('proto.proto.api.v1.Vector3', null, global);
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.StatusRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.StatusRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.StatusRequest.displayName = 'proto.proto.api.v1.StatusRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.StatusStreamRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.StatusStreamRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.StatusStreamRequest.displayName = 'proto.proto.api.v1.StatusStreamRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.StatusResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.StatusResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.StatusResponse.displayName = 'proto.proto.api.v1.StatusResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.StatusStreamResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.StatusStreamResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.StatusStreamResponse.displayName = 'proto.proto.api.v1.StatusStreamResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.Status = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.Status, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.Status.displayName = 'proto.proto.api.v1.Status';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ComponentConfig = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ComponentConfig, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ComponentConfig.displayName = 'proto.proto.api.v1.ComponentConfig';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ConfigRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ConfigRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ConfigRequest.displayName = 'proto.proto.api.v1.ConfigRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ConfigResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.ConfigResponse.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.ConfigResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ConfigResponse.displayName = 'proto.proto.api.v1.ConfigResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.DoActionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.DoActionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.DoActionRequest.displayName = 'proto.proto.api.v1.DoActionRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.DoActionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.DoActionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.DoActionResponse.displayName = 'proto.proto.api.v1.DoActionResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.GantryStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.GantryStatus.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.GantryStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.GantryStatus.displayName = 'proto.proto.api.v1.GantryStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ArmStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ArmStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ArmStatus.displayName = 'proto.proto.api.v1.ArmStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.Pose = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.Pose, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.Pose.displayName = 'proto.proto.api.v1.Pose';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.JointPositions = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.JointPositions.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.JointPositions, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.JointPositions.displayName = 'proto.proto.api.v1.JointPositions';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.FrameConfig = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.FrameConfig, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.FrameConfig.displayName = 'proto.proto.api.v1.FrameConfig';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.FrameSystemConfig = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.FrameSystemConfig, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.FrameSystemConfig.displayName = 'proto.proto.api.v1.FrameSystemConfig';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.FrameServiceConfigRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.FrameServiceConfigRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.FrameServiceConfigRequest.displayName = 'proto.proto.api.v1.FrameServiceConfigRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.FrameServiceConfigResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.FrameServiceConfigResponse.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.FrameServiceConfigResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.FrameServiceConfigResponse.displayName = 'proto.proto.api.v1.FrameServiceConfigResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.Vector3 = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.Vector3, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.Vector3.displayName = 'proto.proto.api.v1.Vector3';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.SensorStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.SensorStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.SensorStatus.displayName = 'proto.proto.api.v1.SensorStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.SensorReadingsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.SensorReadingsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.SensorReadingsRequest.displayName = 'proto.proto.api.v1.SensorReadingsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.SensorReadingsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.SensorReadingsResponse.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.SensorReadingsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.SensorReadingsResponse.displayName = 'proto.proto.api.v1.SensorReadingsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ExecuteFunctionRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ExecuteFunctionRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ExecuteFunctionRequest.displayName = 'proto.proto.api.v1.ExecuteFunctionRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ExecuteFunctionResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.ExecuteFunctionResponse.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.ExecuteFunctionResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ExecuteFunctionResponse.displayName = 'proto.proto.api.v1.ExecuteFunctionResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ExecuteSourceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ExecuteSourceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ExecuteSourceRequest.displayName = 'proto.proto.api.v1.ExecuteSourceRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ExecuteSourceResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.ExecuteSourceResponse.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.ExecuteSourceResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ExecuteSourceResponse.displayName = 'proto.proto.api.v1.ExecuteSourceResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.MotorStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.MotorStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.MotorStatus.displayName = 'proto.proto.api.v1.MotorStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ServoStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ServoStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ServoStatus.displayName = 'proto.proto.api.v1.ServoStatus';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ResourceRunCommandRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ResourceRunCommandRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ResourceRunCommandRequest.displayName = 'proto.proto.api.v1.ResourceRunCommandRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ResourceRunCommandResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ResourceRunCommandResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ResourceRunCommandResponse.displayName = 'proto.proto.api.v1.ResourceRunCommandResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceModeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceModeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceModeRequest.displayName = 'proto.proto.api.v1.NavigationServiceModeRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceModeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceModeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceModeResponse.displayName = 'proto.proto.api.v1.NavigationServiceModeResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceSetModeRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceSetModeRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceSetModeRequest.displayName = 'proto.proto.api.v1.NavigationServiceSetModeRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceSetModeResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceSetModeResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceSetModeResponse.displayName = 'proto.proto.api.v1.NavigationServiceSetModeResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceWaypoint = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceWaypoint, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceWaypoint.displayName = 'proto.proto.api.v1.NavigationServiceWaypoint';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceLocationRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceLocationRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceLocationRequest.displayName = 'proto.proto.api.v1.NavigationServiceLocationRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceLocationResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceLocationResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceLocationResponse.displayName = 'proto.proto.api.v1.NavigationServiceLocationResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceWaypointsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceWaypointsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceWaypointsRequest.displayName = 'proto.proto.api.v1.NavigationServiceWaypointsRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.NavigationServiceWaypointsResponse.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceWaypointsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceWaypointsResponse.displayName = 'proto.proto.api.v1.NavigationServiceWaypointsResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceAddWaypointRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceAddWaypointRequest.displayName = 'proto.proto.api.v1.NavigationServiceAddWaypointRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceAddWaypointResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceAddWaypointResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceAddWaypointResponse.displayName = 'proto.proto.api.v1.NavigationServiceAddWaypointResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceRemoveWaypointRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.displayName = 'proto.proto.api.v1.NavigationServiceRemoveWaypointRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.NavigationServiceRemoveWaypointResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.displayName = 'proto.proto.api.v1.NavigationServiceRemoveWaypointResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.displayName = 'proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.displayName = 'proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.InputControllerEvent = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.proto.api.v1.InputControllerEvent, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.InputControllerEvent.displayName = 'proto.proto.api.v1.InputControllerEvent';
}
/**
 * Generated by JsPbCodeGenerator.
 * @param {Array=} opt_data Optional initial data array, typically from a
 * server response, or constructed directly in Javascript. The array is used
 * in place and becomes part of the constructed object. It is not cloned.
 * If no data is provided, the constructed object will be empty, but still
 * valid.
 * @extends {jspb.Message}
 * @constructor
 */
proto.proto.api.v1.InputControllerStatus = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.proto.api.v1.InputControllerStatus.repeatedFields_, null);
};
goog.inherits(proto.proto.api.v1.InputControllerStatus, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.proto.api.v1.InputControllerStatus.displayName = 'proto.proto.api.v1.InputControllerStatus';
}



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.StatusRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.StatusRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.StatusRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.StatusRequest}
 */
proto.proto.api.v1.StatusRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.StatusRequest;
  return proto.proto.api.v1.StatusRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.StatusRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.StatusRequest}
 */
proto.proto.api.v1.StatusRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.StatusRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.StatusRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.StatusRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.StatusStreamRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.StatusStreamRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.StatusStreamRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusStreamRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    every: (f = msg.getEvery()) && google_protobuf_duration_pb.Duration.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.StatusStreamRequest}
 */
proto.proto.api.v1.StatusStreamRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.StatusStreamRequest;
  return proto.proto.api.v1.StatusStreamRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.StatusStreamRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.StatusStreamRequest}
 */
proto.proto.api.v1.StatusStreamRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_protobuf_duration_pb.Duration;
      reader.readMessage(value,google_protobuf_duration_pb.Duration.deserializeBinaryFromReader);
      msg.setEvery(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.StatusStreamRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.StatusStreamRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.StatusStreamRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusStreamRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEvery();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      google_protobuf_duration_pb.Duration.serializeBinaryToWriter
    );
  }
};


/**
 * optional google.protobuf.Duration every = 1;
 * @return {?proto.google.protobuf.Duration}
 */
proto.proto.api.v1.StatusStreamRequest.prototype.getEvery = function() {
  return /** @type{?proto.google.protobuf.Duration} */ (
    jspb.Message.getWrapperField(this, google_protobuf_duration_pb.Duration, 1));
};


/**
 * @param {?proto.google.protobuf.Duration|undefined} value
 * @return {!proto.proto.api.v1.StatusStreamRequest} returns this
*/
proto.proto.api.v1.StatusStreamRequest.prototype.setEvery = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.StatusStreamRequest} returns this
 */
proto.proto.api.v1.StatusStreamRequest.prototype.clearEvery = function() {
  return this.setEvery(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.StatusStreamRequest.prototype.hasEvery = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.StatusResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.StatusResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.StatusResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    status: (f = msg.getStatus()) && proto.proto.api.v1.Status.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.StatusResponse}
 */
proto.proto.api.v1.StatusResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.StatusResponse;
  return proto.proto.api.v1.StatusResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.StatusResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.StatusResponse}
 */
proto.proto.api.v1.StatusResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.proto.api.v1.Status;
      reader.readMessage(value,proto.proto.api.v1.Status.deserializeBinaryFromReader);
      msg.setStatus(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.StatusResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.StatusResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.StatusResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.proto.api.v1.Status.serializeBinaryToWriter
    );
  }
};


/**
 * optional Status status = 1;
 * @return {?proto.proto.api.v1.Status}
 */
proto.proto.api.v1.StatusResponse.prototype.getStatus = function() {
  return /** @type{?proto.proto.api.v1.Status} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.Status, 1));
};


/**
 * @param {?proto.proto.api.v1.Status|undefined} value
 * @return {!proto.proto.api.v1.StatusResponse} returns this
*/
proto.proto.api.v1.StatusResponse.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.StatusResponse} returns this
 */
proto.proto.api.v1.StatusResponse.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.StatusResponse.prototype.hasStatus = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.StatusStreamResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.StatusStreamResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.StatusStreamResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusStreamResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    status: (f = msg.getStatus()) && proto.proto.api.v1.Status.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.StatusStreamResponse}
 */
proto.proto.api.v1.StatusStreamResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.StatusStreamResponse;
  return proto.proto.api.v1.StatusStreamResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.StatusStreamResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.StatusStreamResponse}
 */
proto.proto.api.v1.StatusStreamResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.proto.api.v1.Status;
      reader.readMessage(value,proto.proto.api.v1.Status.deserializeBinaryFromReader);
      msg.setStatus(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.StatusStreamResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.StatusStreamResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.StatusStreamResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.StatusStreamResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getStatus();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.proto.api.v1.Status.serializeBinaryToWriter
    );
  }
};


/**
 * optional Status status = 1;
 * @return {?proto.proto.api.v1.Status}
 */
proto.proto.api.v1.StatusStreamResponse.prototype.getStatus = function() {
  return /** @type{?proto.proto.api.v1.Status} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.Status, 1));
};


/**
 * @param {?proto.proto.api.v1.Status|undefined} value
 * @return {!proto.proto.api.v1.StatusStreamResponse} returns this
*/
proto.proto.api.v1.StatusStreamResponse.prototype.setStatus = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.StatusStreamResponse} returns this
 */
proto.proto.api.v1.StatusStreamResponse.prototype.clearStatus = function() {
  return this.setStatus(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.StatusStreamResponse.prototype.hasStatus = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.Status.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.Status.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.Status} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.Status.toObject = function(includeInstance, msg) {
  var f, obj = {
    armsMap: (f = msg.getArmsMap()) ? f.toObject(includeInstance, proto.proto.api.v1.ArmStatus.toObject) : [],
    basesMap: (f = msg.getBasesMap()) ? f.toObject(includeInstance, undefined) : [],
    grippersMap: (f = msg.getGrippersMap()) ? f.toObject(includeInstance, undefined) : [],
    boardsMap: (f = msg.getBoardsMap()) ? f.toObject(includeInstance, proto.proto.api.common.v1.BoardStatus.toObject) : [],
    camerasMap: (f = msg.getCamerasMap()) ? f.toObject(includeInstance, undefined) : [],
    sensorsMap: (f = msg.getSensorsMap()) ? f.toObject(includeInstance, proto.proto.api.v1.SensorStatus.toObject) : [],
    functionsMap: (f = msg.getFunctionsMap()) ? f.toObject(includeInstance, undefined) : [],
    servosMap: (f = msg.getServosMap()) ? f.toObject(includeInstance, proto.proto.api.v1.ServoStatus.toObject) : [],
    motorsMap: (f = msg.getMotorsMap()) ? f.toObject(includeInstance, proto.proto.api.v1.MotorStatus.toObject) : [],
    servicesMap: (f = msg.getServicesMap()) ? f.toObject(includeInstance, undefined) : [],
    inputControllersMap: (f = msg.getInputControllersMap()) ? f.toObject(includeInstance, proto.proto.api.v1.InputControllerStatus.toObject) : [],
    gantriesMap: (f = msg.getGantriesMap()) ? f.toObject(includeInstance, proto.proto.api.v1.GantryStatus.toObject) : []
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.Status}
 */
proto.proto.api.v1.Status.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.Status;
  return proto.proto.api.v1.Status.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.Status} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.Status}
 */
proto.proto.api.v1.Status.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = msg.getArmsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readMessage, proto.proto.api.v1.ArmStatus.deserializeBinaryFromReader, "", new proto.proto.api.v1.ArmStatus());
         });
      break;
    case 2:
      var value = msg.getBasesMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readBool, null, "", false);
         });
      break;
    case 3:
      var value = msg.getGrippersMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readBool, null, "", false);
         });
      break;
    case 4:
      var value = msg.getBoardsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readMessage, proto.proto.api.common.v1.BoardStatus.deserializeBinaryFromReader, "", new proto.proto.api.common.v1.BoardStatus());
         });
      break;
    case 5:
      var value = msg.getCamerasMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readBool, null, "", false);
         });
      break;
    case 6:
      var value = msg.getSensorsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readMessage, proto.proto.api.v1.SensorStatus.deserializeBinaryFromReader, "", new proto.proto.api.v1.SensorStatus());
         });
      break;
    case 7:
      var value = msg.getFunctionsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readBool, null, "", false);
         });
      break;
    case 8:
      var value = msg.getServosMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readMessage, proto.proto.api.v1.ServoStatus.deserializeBinaryFromReader, "", new proto.proto.api.v1.ServoStatus());
         });
      break;
    case 9:
      var value = msg.getMotorsMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readMessage, proto.proto.api.v1.MotorStatus.deserializeBinaryFromReader, "", new proto.proto.api.v1.MotorStatus());
         });
      break;
    case 10:
      var value = msg.getServicesMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readBool, null, "", false);
         });
      break;
    case 11:
      var value = msg.getInputControllersMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readMessage, proto.proto.api.v1.InputControllerStatus.deserializeBinaryFromReader, "", new proto.proto.api.v1.InputControllerStatus());
         });
      break;
    case 12:
      var value = msg.getGantriesMap();
      reader.readMessage(value, function(message, reader) {
        jspb.Map.deserializeBinary(message, reader, jspb.BinaryReader.prototype.readString, jspb.BinaryReader.prototype.readMessage, proto.proto.api.v1.GantryStatus.deserializeBinaryFromReader, "", new proto.proto.api.v1.GantryStatus());
         });
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.Status.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.Status.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.Status} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.Status.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getArmsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(1, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeMessage, proto.proto.api.v1.ArmStatus.serializeBinaryToWriter);
  }
  f = message.getBasesMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(2, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeBool);
  }
  f = message.getGrippersMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(3, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeBool);
  }
  f = message.getBoardsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(4, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeMessage, proto.proto.api.common.v1.BoardStatus.serializeBinaryToWriter);
  }
  f = message.getCamerasMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(5, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeBool);
  }
  f = message.getSensorsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(6, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeMessage, proto.proto.api.v1.SensorStatus.serializeBinaryToWriter);
  }
  f = message.getFunctionsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(7, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeBool);
  }
  f = message.getServosMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(8, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeMessage, proto.proto.api.v1.ServoStatus.serializeBinaryToWriter);
  }
  f = message.getMotorsMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(9, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeMessage, proto.proto.api.v1.MotorStatus.serializeBinaryToWriter);
  }
  f = message.getServicesMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(10, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeBool);
  }
  f = message.getInputControllersMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(11, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeMessage, proto.proto.api.v1.InputControllerStatus.serializeBinaryToWriter);
  }
  f = message.getGantriesMap(true);
  if (f && f.getLength() > 0) {
    f.serializeBinary(12, writer, jspb.BinaryWriter.prototype.writeString, jspb.BinaryWriter.prototype.writeMessage, proto.proto.api.v1.GantryStatus.serializeBinaryToWriter);
  }
};


/**
 * map<string, ArmStatus> arms = 1;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!proto.proto.api.v1.ArmStatus>}
 */
proto.proto.api.v1.Status.prototype.getArmsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!proto.proto.api.v1.ArmStatus>} */ (
      jspb.Message.getMapField(this, 1, opt_noLazyCreate,
      proto.proto.api.v1.ArmStatus));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearArmsMap = function() {
  this.getArmsMap().clear();
  return this;};


/**
 * map<string, bool> bases = 2;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,boolean>}
 */
proto.proto.api.v1.Status.prototype.getBasesMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,boolean>} */ (
      jspb.Message.getMapField(this, 2, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearBasesMap = function() {
  this.getBasesMap().clear();
  return this;};


/**
 * map<string, bool> grippers = 3;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,boolean>}
 */
proto.proto.api.v1.Status.prototype.getGrippersMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,boolean>} */ (
      jspb.Message.getMapField(this, 3, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearGrippersMap = function() {
  this.getGrippersMap().clear();
  return this;};


/**
 * map<string, proto.api.common.v1.BoardStatus> boards = 4;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!proto.proto.api.common.v1.BoardStatus>}
 */
proto.proto.api.v1.Status.prototype.getBoardsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!proto.proto.api.common.v1.BoardStatus>} */ (
      jspb.Message.getMapField(this, 4, opt_noLazyCreate,
      proto.proto.api.common.v1.BoardStatus));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearBoardsMap = function() {
  this.getBoardsMap().clear();
  return this;};


/**
 * map<string, bool> cameras = 5;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,boolean>}
 */
proto.proto.api.v1.Status.prototype.getCamerasMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,boolean>} */ (
      jspb.Message.getMapField(this, 5, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearCamerasMap = function() {
  this.getCamerasMap().clear();
  return this;};


/**
 * map<string, SensorStatus> sensors = 6;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!proto.proto.api.v1.SensorStatus>}
 */
proto.proto.api.v1.Status.prototype.getSensorsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!proto.proto.api.v1.SensorStatus>} */ (
      jspb.Message.getMapField(this, 6, opt_noLazyCreate,
      proto.proto.api.v1.SensorStatus));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearSensorsMap = function() {
  this.getSensorsMap().clear();
  return this;};


/**
 * map<string, bool> functions = 7;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,boolean>}
 */
proto.proto.api.v1.Status.prototype.getFunctionsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,boolean>} */ (
      jspb.Message.getMapField(this, 7, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearFunctionsMap = function() {
  this.getFunctionsMap().clear();
  return this;};


/**
 * map<string, ServoStatus> servos = 8;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!proto.proto.api.v1.ServoStatus>}
 */
proto.proto.api.v1.Status.prototype.getServosMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!proto.proto.api.v1.ServoStatus>} */ (
      jspb.Message.getMapField(this, 8, opt_noLazyCreate,
      proto.proto.api.v1.ServoStatus));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearServosMap = function() {
  this.getServosMap().clear();
  return this;};


/**
 * map<string, MotorStatus> motors = 9;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!proto.proto.api.v1.MotorStatus>}
 */
proto.proto.api.v1.Status.prototype.getMotorsMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!proto.proto.api.v1.MotorStatus>} */ (
      jspb.Message.getMapField(this, 9, opt_noLazyCreate,
      proto.proto.api.v1.MotorStatus));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearMotorsMap = function() {
  this.getMotorsMap().clear();
  return this;};


/**
 * map<string, bool> services = 10;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,boolean>}
 */
proto.proto.api.v1.Status.prototype.getServicesMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,boolean>} */ (
      jspb.Message.getMapField(this, 10, opt_noLazyCreate,
      null));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearServicesMap = function() {
  this.getServicesMap().clear();
  return this;};


/**
 * map<string, InputControllerStatus> input_controllers = 11;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!proto.proto.api.v1.InputControllerStatus>}
 */
proto.proto.api.v1.Status.prototype.getInputControllersMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!proto.proto.api.v1.InputControllerStatus>} */ (
      jspb.Message.getMapField(this, 11, opt_noLazyCreate,
      proto.proto.api.v1.InputControllerStatus));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearInputControllersMap = function() {
  this.getInputControllersMap().clear();
  return this;};


/**
 * map<string, GantryStatus> gantries = 12;
 * @param {boolean=} opt_noLazyCreate Do not create the map if
 * empty, instead returning `undefined`
 * @return {!jspb.Map<string,!proto.proto.api.v1.GantryStatus>}
 */
proto.proto.api.v1.Status.prototype.getGantriesMap = function(opt_noLazyCreate) {
  return /** @type {!jspb.Map<string,!proto.proto.api.v1.GantryStatus>} */ (
      jspb.Message.getMapField(this, 12, opt_noLazyCreate,
      proto.proto.api.v1.GantryStatus));
};


/**
 * Clears values from the map. The map will be non-null.
 * @return {!proto.proto.api.v1.Status} returns this
 */
proto.proto.api.v1.Status.prototype.clearGantriesMap = function() {
  this.getGantriesMap().clear();
  return this;};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ComponentConfig.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ComponentConfig.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ComponentConfig} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ComponentConfig.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    type: jspb.Message.getFieldWithDefault(msg, 2, ""),
    parent: jspb.Message.getFieldWithDefault(msg, 3, ""),
    pose: (f = msg.getPose()) && proto.proto.api.v1.Pose.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ComponentConfig}
 */
proto.proto.api.v1.ComponentConfig.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ComponentConfig;
  return proto.proto.api.v1.ComponentConfig.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ComponentConfig} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ComponentConfig}
 */
proto.proto.api.v1.ComponentConfig.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setType(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setParent(value);
      break;
    case 4:
      var value = new proto.proto.api.v1.Pose;
      reader.readMessage(value,proto.proto.api.v1.Pose.deserializeBinaryFromReader);
      msg.setPose(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ComponentConfig.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ComponentConfig.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ComponentConfig} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ComponentConfig.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getType();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getPose();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.proto.api.v1.Pose.serializeBinaryToWriter
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.proto.api.v1.ComponentConfig.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ComponentConfig} returns this
 */
proto.proto.api.v1.ComponentConfig.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string type = 2;
 * @return {string}
 */
proto.proto.api.v1.ComponentConfig.prototype.getType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ComponentConfig} returns this
 */
proto.proto.api.v1.ComponentConfig.prototype.setType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string parent = 3;
 * @return {string}
 */
proto.proto.api.v1.ComponentConfig.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ComponentConfig} returns this
 */
proto.proto.api.v1.ComponentConfig.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional Pose pose = 4;
 * @return {?proto.proto.api.v1.Pose}
 */
proto.proto.api.v1.ComponentConfig.prototype.getPose = function() {
  return /** @type{?proto.proto.api.v1.Pose} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.Pose, 4));
};


/**
 * @param {?proto.proto.api.v1.Pose|undefined} value
 * @return {!proto.proto.api.v1.ComponentConfig} returns this
*/
proto.proto.api.v1.ComponentConfig.prototype.setPose = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.ComponentConfig} returns this
 */
proto.proto.api.v1.ComponentConfig.prototype.clearPose = function() {
  return this.setPose(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.ComponentConfig.prototype.hasPose = function() {
  return jspb.Message.getField(this, 4) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ConfigRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ConfigRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ConfigRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ConfigRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ConfigRequest}
 */
proto.proto.api.v1.ConfigRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ConfigRequest;
  return proto.proto.api.v1.ConfigRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ConfigRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ConfigRequest}
 */
proto.proto.api.v1.ConfigRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ConfigRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ConfigRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ConfigRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ConfigRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.ConfigResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ConfigResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ConfigResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ConfigResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ConfigResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    componentsList: jspb.Message.toObjectList(msg.getComponentsList(),
    proto.proto.api.v1.ComponentConfig.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ConfigResponse}
 */
proto.proto.api.v1.ConfigResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ConfigResponse;
  return proto.proto.api.v1.ConfigResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ConfigResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ConfigResponse}
 */
proto.proto.api.v1.ConfigResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.proto.api.v1.ComponentConfig;
      reader.readMessage(value,proto.proto.api.v1.ComponentConfig.deserializeBinaryFromReader);
      msg.addComponents(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ConfigResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ConfigResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ConfigResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ConfigResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getComponentsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.proto.api.v1.ComponentConfig.serializeBinaryToWriter
    );
  }
};


/**
 * repeated ComponentConfig components = 1;
 * @return {!Array<!proto.proto.api.v1.ComponentConfig>}
 */
proto.proto.api.v1.ConfigResponse.prototype.getComponentsList = function() {
  return /** @type{!Array<!proto.proto.api.v1.ComponentConfig>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.proto.api.v1.ComponentConfig, 1));
};


/**
 * @param {!Array<!proto.proto.api.v1.ComponentConfig>} value
 * @return {!proto.proto.api.v1.ConfigResponse} returns this
*/
proto.proto.api.v1.ConfigResponse.prototype.setComponentsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.proto.api.v1.ComponentConfig=} opt_value
 * @param {number=} opt_index
 * @return {!proto.proto.api.v1.ComponentConfig}
 */
proto.proto.api.v1.ConfigResponse.prototype.addComponents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.proto.api.v1.ComponentConfig, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.ConfigResponse} returns this
 */
proto.proto.api.v1.ConfigResponse.prototype.clearComponentsList = function() {
  return this.setComponentsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.DoActionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.DoActionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.DoActionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.DoActionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.DoActionRequest}
 */
proto.proto.api.v1.DoActionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.DoActionRequest;
  return proto.proto.api.v1.DoActionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.DoActionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.DoActionRequest}
 */
proto.proto.api.v1.DoActionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.DoActionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.DoActionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.DoActionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.DoActionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.proto.api.v1.DoActionRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.DoActionRequest} returns this
 */
proto.proto.api.v1.DoActionRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.DoActionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.DoActionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.DoActionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.DoActionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.DoActionResponse}
 */
proto.proto.api.v1.DoActionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.DoActionResponse;
  return proto.proto.api.v1.DoActionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.DoActionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.DoActionResponse}
 */
proto.proto.api.v1.DoActionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.DoActionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.DoActionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.DoActionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.DoActionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.GantryStatus.repeatedFields_ = [1,2];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.GantryStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.GantryStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.GantryStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.GantryStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    positionsList: (f = jspb.Message.getRepeatedFloatingPointField(msg, 1)) == null ? undefined : f,
    lengthsList: (f = jspb.Message.getRepeatedFloatingPointField(msg, 2)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.GantryStatus}
 */
proto.proto.api.v1.GantryStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.GantryStatus;
  return proto.proto.api.v1.GantryStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.GantryStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.GantryStatus}
 */
proto.proto.api.v1.GantryStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var values = /** @type {!Array<number>} */ (reader.isDelimited() ? reader.readPackedDouble() : [reader.readDouble()]);
      for (var i = 0; i < values.length; i++) {
        msg.addPositions(values[i]);
      }
      break;
    case 2:
      var values = /** @type {!Array<number>} */ (reader.isDelimited() ? reader.readPackedDouble() : [reader.readDouble()]);
      for (var i = 0; i < values.length; i++) {
        msg.addLengths(values[i]);
      }
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.GantryStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.GantryStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.GantryStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.GantryStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPositionsList();
  if (f.length > 0) {
    writer.writePackedDouble(
      1,
      f
    );
  }
  f = message.getLengthsList();
  if (f.length > 0) {
    writer.writePackedDouble(
      2,
      f
    );
  }
};


/**
 * repeated double positions = 1;
 * @return {!Array<number>}
 */
proto.proto.api.v1.GantryStatus.prototype.getPositionsList = function() {
  return /** @type {!Array<number>} */ (jspb.Message.getRepeatedFloatingPointField(this, 1));
};


/**
 * @param {!Array<number>} value
 * @return {!proto.proto.api.v1.GantryStatus} returns this
 */
proto.proto.api.v1.GantryStatus.prototype.setPositionsList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {number} value
 * @param {number=} opt_index
 * @return {!proto.proto.api.v1.GantryStatus} returns this
 */
proto.proto.api.v1.GantryStatus.prototype.addPositions = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.GantryStatus} returns this
 */
proto.proto.api.v1.GantryStatus.prototype.clearPositionsList = function() {
  return this.setPositionsList([]);
};


/**
 * repeated double lengths = 2;
 * @return {!Array<number>}
 */
proto.proto.api.v1.GantryStatus.prototype.getLengthsList = function() {
  return /** @type {!Array<number>} */ (jspb.Message.getRepeatedFloatingPointField(this, 2));
};


/**
 * @param {!Array<number>} value
 * @return {!proto.proto.api.v1.GantryStatus} returns this
 */
proto.proto.api.v1.GantryStatus.prototype.setLengthsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {number} value
 * @param {number=} opt_index
 * @return {!proto.proto.api.v1.GantryStatus} returns this
 */
proto.proto.api.v1.GantryStatus.prototype.addLengths = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.GantryStatus} returns this
 */
proto.proto.api.v1.GantryStatus.prototype.clearLengthsList = function() {
  return this.setLengthsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ArmStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ArmStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ArmStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ArmStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    gridPosition: (f = msg.getGridPosition()) && proto.proto.api.v1.Pose.toObject(includeInstance, f),
    jointPositions: (f = msg.getJointPositions()) && proto.proto.api.v1.JointPositions.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ArmStatus}
 */
proto.proto.api.v1.ArmStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ArmStatus;
  return proto.proto.api.v1.ArmStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ArmStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ArmStatus}
 */
proto.proto.api.v1.ArmStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.proto.api.v1.Pose;
      reader.readMessage(value,proto.proto.api.v1.Pose.deserializeBinaryFromReader);
      msg.setGridPosition(value);
      break;
    case 2:
      var value = new proto.proto.api.v1.JointPositions;
      reader.readMessage(value,proto.proto.api.v1.JointPositions.deserializeBinaryFromReader);
      msg.setJointPositions(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ArmStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ArmStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ArmStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ArmStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getGridPosition();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto.proto.api.v1.Pose.serializeBinaryToWriter
    );
  }
  f = message.getJointPositions();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.proto.api.v1.JointPositions.serializeBinaryToWriter
    );
  }
};


/**
 * optional Pose grid_position = 1;
 * @return {?proto.proto.api.v1.Pose}
 */
proto.proto.api.v1.ArmStatus.prototype.getGridPosition = function() {
  return /** @type{?proto.proto.api.v1.Pose} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.Pose, 1));
};


/**
 * @param {?proto.proto.api.v1.Pose|undefined} value
 * @return {!proto.proto.api.v1.ArmStatus} returns this
*/
proto.proto.api.v1.ArmStatus.prototype.setGridPosition = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.ArmStatus} returns this
 */
proto.proto.api.v1.ArmStatus.prototype.clearGridPosition = function() {
  return this.setGridPosition(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.ArmStatus.prototype.hasGridPosition = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional JointPositions joint_positions = 2;
 * @return {?proto.proto.api.v1.JointPositions}
 */
proto.proto.api.v1.ArmStatus.prototype.getJointPositions = function() {
  return /** @type{?proto.proto.api.v1.JointPositions} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.JointPositions, 2));
};


/**
 * @param {?proto.proto.api.v1.JointPositions|undefined} value
 * @return {!proto.proto.api.v1.ArmStatus} returns this
*/
proto.proto.api.v1.ArmStatus.prototype.setJointPositions = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.ArmStatus} returns this
 */
proto.proto.api.v1.ArmStatus.prototype.clearJointPositions = function() {
  return this.setJointPositions(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.ArmStatus.prototype.hasJointPositions = function() {
  return jspb.Message.getField(this, 2) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.Pose.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.Pose.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.Pose} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.Pose.toObject = function(includeInstance, msg) {
  var f, obj = {
    x: jspb.Message.getFloatingPointFieldWithDefault(msg, 1, 0.0),
    y: jspb.Message.getFloatingPointFieldWithDefault(msg, 2, 0.0),
    z: jspb.Message.getFloatingPointFieldWithDefault(msg, 3, 0.0),
    oX: jspb.Message.getFloatingPointFieldWithDefault(msg, 4, 0.0),
    oY: jspb.Message.getFloatingPointFieldWithDefault(msg, 5, 0.0),
    oZ: jspb.Message.getFloatingPointFieldWithDefault(msg, 6, 0.0),
    theta: jspb.Message.getFloatingPointFieldWithDefault(msg, 7, 0.0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.Pose}
 */
proto.proto.api.v1.Pose.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.Pose;
  return proto.proto.api.v1.Pose.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.Pose} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.Pose}
 */
proto.proto.api.v1.Pose.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setX(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setY(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setZ(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setOX(value);
      break;
    case 5:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setOY(value);
      break;
    case 6:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setOZ(value);
      break;
    case 7:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setTheta(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.Pose.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.Pose.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.Pose} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.Pose.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getX();
  if (f !== 0.0) {
    writer.writeDouble(
      1,
      f
    );
  }
  f = message.getY();
  if (f !== 0.0) {
    writer.writeDouble(
      2,
      f
    );
  }
  f = message.getZ();
  if (f !== 0.0) {
    writer.writeDouble(
      3,
      f
    );
  }
  f = message.getOX();
  if (f !== 0.0) {
    writer.writeDouble(
      4,
      f
    );
  }
  f = message.getOY();
  if (f !== 0.0) {
    writer.writeDouble(
      5,
      f
    );
  }
  f = message.getOZ();
  if (f !== 0.0) {
    writer.writeDouble(
      6,
      f
    );
  }
  f = message.getTheta();
  if (f !== 0.0) {
    writer.writeDouble(
      7,
      f
    );
  }
};


/**
 * optional double x = 1;
 * @return {number}
 */
proto.proto.api.v1.Pose.prototype.getX = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 1, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Pose} returns this
 */
proto.proto.api.v1.Pose.prototype.setX = function(value) {
  return jspb.Message.setProto3FloatField(this, 1, value);
};


/**
 * optional double y = 2;
 * @return {number}
 */
proto.proto.api.v1.Pose.prototype.getY = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 2, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Pose} returns this
 */
proto.proto.api.v1.Pose.prototype.setY = function(value) {
  return jspb.Message.setProto3FloatField(this, 2, value);
};


/**
 * optional double z = 3;
 * @return {number}
 */
proto.proto.api.v1.Pose.prototype.getZ = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 3, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Pose} returns this
 */
proto.proto.api.v1.Pose.prototype.setZ = function(value) {
  return jspb.Message.setProto3FloatField(this, 3, value);
};


/**
 * optional double o_x = 4;
 * @return {number}
 */
proto.proto.api.v1.Pose.prototype.getOX = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 4, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Pose} returns this
 */
proto.proto.api.v1.Pose.prototype.setOX = function(value) {
  return jspb.Message.setProto3FloatField(this, 4, value);
};


/**
 * optional double o_y = 5;
 * @return {number}
 */
proto.proto.api.v1.Pose.prototype.getOY = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 5, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Pose} returns this
 */
proto.proto.api.v1.Pose.prototype.setOY = function(value) {
  return jspb.Message.setProto3FloatField(this, 5, value);
};


/**
 * optional double o_z = 6;
 * @return {number}
 */
proto.proto.api.v1.Pose.prototype.getOZ = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 6, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Pose} returns this
 */
proto.proto.api.v1.Pose.prototype.setOZ = function(value) {
  return jspb.Message.setProto3FloatField(this, 6, value);
};


/**
 * optional double theta = 7;
 * @return {number}
 */
proto.proto.api.v1.Pose.prototype.getTheta = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 7, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Pose} returns this
 */
proto.proto.api.v1.Pose.prototype.setTheta = function(value) {
  return jspb.Message.setProto3FloatField(this, 7, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.JointPositions.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.JointPositions.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.JointPositions.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.JointPositions} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.JointPositions.toObject = function(includeInstance, msg) {
  var f, obj = {
    degreesList: (f = jspb.Message.getRepeatedFloatingPointField(msg, 1)) == null ? undefined : f
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.JointPositions}
 */
proto.proto.api.v1.JointPositions.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.JointPositions;
  return proto.proto.api.v1.JointPositions.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.JointPositions} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.JointPositions}
 */
proto.proto.api.v1.JointPositions.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var values = /** @type {!Array<number>} */ (reader.isDelimited() ? reader.readPackedDouble() : [reader.readDouble()]);
      for (var i = 0; i < values.length; i++) {
        msg.addDegrees(values[i]);
      }
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.JointPositions.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.JointPositions.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.JointPositions} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.JointPositions.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getDegreesList();
  if (f.length > 0) {
    writer.writePackedDouble(
      1,
      f
    );
  }
};


/**
 * repeated double degrees = 1;
 * @return {!Array<number>}
 */
proto.proto.api.v1.JointPositions.prototype.getDegreesList = function() {
  return /** @type {!Array<number>} */ (jspb.Message.getRepeatedFloatingPointField(this, 1));
};


/**
 * @param {!Array<number>} value
 * @return {!proto.proto.api.v1.JointPositions} returns this
 */
proto.proto.api.v1.JointPositions.prototype.setDegreesList = function(value) {
  return jspb.Message.setField(this, 1, value || []);
};


/**
 * @param {number} value
 * @param {number=} opt_index
 * @return {!proto.proto.api.v1.JointPositions} returns this
 */
proto.proto.api.v1.JointPositions.prototype.addDegrees = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 1, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.JointPositions} returns this
 */
proto.proto.api.v1.JointPositions.prototype.clearDegreesList = function() {
  return this.setDegreesList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.FrameConfig.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.FrameConfig.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.FrameConfig} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameConfig.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    pose: (f = msg.getPose()) && proto.proto.api.v1.Pose.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.FrameConfig}
 */
proto.proto.api.v1.FrameConfig.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.FrameConfig;
  return proto.proto.api.v1.FrameConfig.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.FrameConfig} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.FrameConfig}
 */
proto.proto.api.v1.FrameConfig.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setParent(value);
      break;
    case 2:
      var value = new proto.proto.api.v1.Pose;
      reader.readMessage(value,proto.proto.api.v1.Pose.deserializeBinaryFromReader);
      msg.setPose(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.FrameConfig.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.FrameConfig.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.FrameConfig} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameConfig.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPose();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.proto.api.v1.Pose.serializeBinaryToWriter
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.proto.api.v1.FrameConfig.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.FrameConfig} returns this
 */
proto.proto.api.v1.FrameConfig.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional Pose pose = 2;
 * @return {?proto.proto.api.v1.Pose}
 */
proto.proto.api.v1.FrameConfig.prototype.getPose = function() {
  return /** @type{?proto.proto.api.v1.Pose} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.Pose, 2));
};


/**
 * @param {?proto.proto.api.v1.Pose|undefined} value
 * @return {!proto.proto.api.v1.FrameConfig} returns this
*/
proto.proto.api.v1.FrameConfig.prototype.setPose = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.FrameConfig} returns this
 */
proto.proto.api.v1.FrameConfig.prototype.clearPose = function() {
  return this.setPose(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.FrameConfig.prototype.hasPose = function() {
  return jspb.Message.getField(this, 2) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.FrameSystemConfig.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.FrameSystemConfig} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameSystemConfig.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    frameConfig: (f = msg.getFrameConfig()) && proto.proto.api.v1.FrameConfig.toObject(includeInstance, f),
    modelJson: msg.getModelJson_asB64()
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.FrameSystemConfig}
 */
proto.proto.api.v1.FrameSystemConfig.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.FrameSystemConfig;
  return proto.proto.api.v1.FrameSystemConfig.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.FrameSystemConfig} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.FrameSystemConfig}
 */
proto.proto.api.v1.FrameSystemConfig.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 2:
      var value = new proto.proto.api.v1.FrameConfig;
      reader.readMessage(value,proto.proto.api.v1.FrameConfig.deserializeBinaryFromReader);
      msg.setFrameConfig(value);
      break;
    case 3:
      var value = /** @type {!Uint8Array} */ (reader.readBytes());
      msg.setModelJson(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.FrameSystemConfig.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.FrameSystemConfig} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameSystemConfig.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getFrameConfig();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.proto.api.v1.FrameConfig.serializeBinaryToWriter
    );
  }
  f = message.getModelJson_asU8();
  if (f.length > 0) {
    writer.writeBytes(
      3,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.FrameSystemConfig} returns this
 */
proto.proto.api.v1.FrameSystemConfig.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional FrameConfig frame_config = 2;
 * @return {?proto.proto.api.v1.FrameConfig}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.getFrameConfig = function() {
  return /** @type{?proto.proto.api.v1.FrameConfig} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.FrameConfig, 2));
};


/**
 * @param {?proto.proto.api.v1.FrameConfig|undefined} value
 * @return {!proto.proto.api.v1.FrameSystemConfig} returns this
*/
proto.proto.api.v1.FrameSystemConfig.prototype.setFrameConfig = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.FrameSystemConfig} returns this
 */
proto.proto.api.v1.FrameSystemConfig.prototype.clearFrameConfig = function() {
  return this.setFrameConfig(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.hasFrameConfig = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bytes model_json = 3;
 * @return {string}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.getModelJson = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * optional bytes model_json = 3;
 * This is a type-conversion wrapper around `getModelJson()`
 * @return {string}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.getModelJson_asB64 = function() {
  return /** @type {string} */ (jspb.Message.bytesAsB64(
      this.getModelJson()));
};


/**
 * optional bytes model_json = 3;
 * Note that Uint8Array is not supported on all browsers.
 * @see http://caniuse.com/Uint8Array
 * This is a type-conversion wrapper around `getModelJson()`
 * @return {!Uint8Array}
 */
proto.proto.api.v1.FrameSystemConfig.prototype.getModelJson_asU8 = function() {
  return /** @type {!Uint8Array} */ (jspb.Message.bytesAsU8(
      this.getModelJson()));
};


/**
 * @param {!(string|Uint8Array)} value
 * @return {!proto.proto.api.v1.FrameSystemConfig} returns this
 */
proto.proto.api.v1.FrameSystemConfig.prototype.setModelJson = function(value) {
  return jspb.Message.setProto3BytesField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.FrameServiceConfigRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.FrameServiceConfigRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.FrameServiceConfigRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameServiceConfigRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.FrameServiceConfigRequest}
 */
proto.proto.api.v1.FrameServiceConfigRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.FrameServiceConfigRequest;
  return proto.proto.api.v1.FrameServiceConfigRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.FrameServiceConfigRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.FrameServiceConfigRequest}
 */
proto.proto.api.v1.FrameServiceConfigRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.FrameServiceConfigRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.FrameServiceConfigRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.FrameServiceConfigRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameServiceConfigRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.FrameServiceConfigResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.FrameServiceConfigResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.FrameServiceConfigResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.FrameServiceConfigResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameServiceConfigResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    frameSystemConfigsList: jspb.Message.toObjectList(msg.getFrameSystemConfigsList(),
    proto.proto.api.v1.FrameSystemConfig.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.FrameServiceConfigResponse}
 */
proto.proto.api.v1.FrameServiceConfigResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.FrameServiceConfigResponse;
  return proto.proto.api.v1.FrameServiceConfigResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.FrameServiceConfigResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.FrameServiceConfigResponse}
 */
proto.proto.api.v1.FrameServiceConfigResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.proto.api.v1.FrameSystemConfig;
      reader.readMessage(value,proto.proto.api.v1.FrameSystemConfig.deserializeBinaryFromReader);
      msg.addFrameSystemConfigs(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.FrameServiceConfigResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.FrameServiceConfigResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.FrameServiceConfigResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.FrameServiceConfigResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getFrameSystemConfigsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.proto.api.v1.FrameSystemConfig.serializeBinaryToWriter
    );
  }
};


/**
 * repeated FrameSystemConfig frame_system_configs = 1;
 * @return {!Array<!proto.proto.api.v1.FrameSystemConfig>}
 */
proto.proto.api.v1.FrameServiceConfigResponse.prototype.getFrameSystemConfigsList = function() {
  return /** @type{!Array<!proto.proto.api.v1.FrameSystemConfig>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.proto.api.v1.FrameSystemConfig, 1));
};


/**
 * @param {!Array<!proto.proto.api.v1.FrameSystemConfig>} value
 * @return {!proto.proto.api.v1.FrameServiceConfigResponse} returns this
*/
proto.proto.api.v1.FrameServiceConfigResponse.prototype.setFrameSystemConfigsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.proto.api.v1.FrameSystemConfig=} opt_value
 * @param {number=} opt_index
 * @return {!proto.proto.api.v1.FrameSystemConfig}
 */
proto.proto.api.v1.FrameServiceConfigResponse.prototype.addFrameSystemConfigs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.proto.api.v1.FrameSystemConfig, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.FrameServiceConfigResponse} returns this
 */
proto.proto.api.v1.FrameServiceConfigResponse.prototype.clearFrameSystemConfigsList = function() {
  return this.setFrameSystemConfigsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.Vector3.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.Vector3.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.Vector3} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.Vector3.toObject = function(includeInstance, msg) {
  var f, obj = {
    x: jspb.Message.getFloatingPointFieldWithDefault(msg, 1, 0.0),
    y: jspb.Message.getFloatingPointFieldWithDefault(msg, 2, 0.0),
    z: jspb.Message.getFloatingPointFieldWithDefault(msg, 3, 0.0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.Vector3}
 */
proto.proto.api.v1.Vector3.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.Vector3;
  return proto.proto.api.v1.Vector3.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.Vector3} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.Vector3}
 */
proto.proto.api.v1.Vector3.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setX(value);
      break;
    case 2:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setY(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setZ(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.Vector3.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.Vector3.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.Vector3} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.Vector3.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getX();
  if (f !== 0.0) {
    writer.writeDouble(
      1,
      f
    );
  }
  f = message.getY();
  if (f !== 0.0) {
    writer.writeDouble(
      2,
      f
    );
  }
  f = message.getZ();
  if (f !== 0.0) {
    writer.writeDouble(
      3,
      f
    );
  }
};


/**
 * optional double x = 1;
 * @return {number}
 */
proto.proto.api.v1.Vector3.prototype.getX = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 1, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Vector3} returns this
 */
proto.proto.api.v1.Vector3.prototype.setX = function(value) {
  return jspb.Message.setProto3FloatField(this, 1, value);
};


/**
 * optional double y = 2;
 * @return {number}
 */
proto.proto.api.v1.Vector3.prototype.getY = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 2, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Vector3} returns this
 */
proto.proto.api.v1.Vector3.prototype.setY = function(value) {
  return jspb.Message.setProto3FloatField(this, 2, value);
};


/**
 * optional double z = 3;
 * @return {number}
 */
proto.proto.api.v1.Vector3.prototype.getZ = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 3, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.Vector3} returns this
 */
proto.proto.api.v1.Vector3.prototype.setZ = function(value) {
  return jspb.Message.setProto3FloatField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.SensorStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.SensorStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.SensorStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.SensorStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    type: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.SensorStatus}
 */
proto.proto.api.v1.SensorStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.SensorStatus;
  return proto.proto.api.v1.SensorStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.SensorStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.SensorStatus}
 */
proto.proto.api.v1.SensorStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setType(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.SensorStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.SensorStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.SensorStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.SensorStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getType();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string type = 1;
 * @return {string}
 */
proto.proto.api.v1.SensorStatus.prototype.getType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.SensorStatus} returns this
 */
proto.proto.api.v1.SensorStatus.prototype.setType = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.SensorReadingsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.SensorReadingsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.SensorReadingsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.SensorReadingsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.SensorReadingsRequest}
 */
proto.proto.api.v1.SensorReadingsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.SensorReadingsRequest;
  return proto.proto.api.v1.SensorReadingsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.SensorReadingsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.SensorReadingsRequest}
 */
proto.proto.api.v1.SensorReadingsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.SensorReadingsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.SensorReadingsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.SensorReadingsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.SensorReadingsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.proto.api.v1.SensorReadingsRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.SensorReadingsRequest} returns this
 */
proto.proto.api.v1.SensorReadingsRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.SensorReadingsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.SensorReadingsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.SensorReadingsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.SensorReadingsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.SensorReadingsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    readingsList: jspb.Message.toObjectList(msg.getReadingsList(),
    google_protobuf_struct_pb.Value.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.SensorReadingsResponse}
 */
proto.proto.api.v1.SensorReadingsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.SensorReadingsResponse;
  return proto.proto.api.v1.SensorReadingsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.SensorReadingsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.SensorReadingsResponse}
 */
proto.proto.api.v1.SensorReadingsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_protobuf_struct_pb.Value;
      reader.readMessage(value,google_protobuf_struct_pb.Value.deserializeBinaryFromReader);
      msg.addReadings(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.SensorReadingsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.SensorReadingsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.SensorReadingsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.SensorReadingsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getReadingsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_protobuf_struct_pb.Value.serializeBinaryToWriter
    );
  }
};


/**
 * repeated google.protobuf.Value readings = 1;
 * @return {!Array<!proto.google.protobuf.Value>}
 */
proto.proto.api.v1.SensorReadingsResponse.prototype.getReadingsList = function() {
  return /** @type{!Array<!proto.google.protobuf.Value>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_protobuf_struct_pb.Value, 1));
};


/**
 * @param {!Array<!proto.google.protobuf.Value>} value
 * @return {!proto.proto.api.v1.SensorReadingsResponse} returns this
*/
proto.proto.api.v1.SensorReadingsResponse.prototype.setReadingsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.protobuf.Value=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.protobuf.Value}
 */
proto.proto.api.v1.SensorReadingsResponse.prototype.addReadings = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.protobuf.Value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.SensorReadingsResponse} returns this
 */
proto.proto.api.v1.SensorReadingsResponse.prototype.clearReadingsList = function() {
  return this.setReadingsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ExecuteFunctionRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ExecuteFunctionRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ExecuteFunctionRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteFunctionRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ExecuteFunctionRequest}
 */
proto.proto.api.v1.ExecuteFunctionRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ExecuteFunctionRequest;
  return proto.proto.api.v1.ExecuteFunctionRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ExecuteFunctionRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ExecuteFunctionRequest}
 */
proto.proto.api.v1.ExecuteFunctionRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ExecuteFunctionRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ExecuteFunctionRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ExecuteFunctionRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteFunctionRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.proto.api.v1.ExecuteFunctionRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ExecuteFunctionRequest} returns this
 */
proto.proto.api.v1.ExecuteFunctionRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.ExecuteFunctionResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ExecuteFunctionResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ExecuteFunctionResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteFunctionResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    google_protobuf_struct_pb.Value.toObject, includeInstance),
    stdOut: jspb.Message.getFieldWithDefault(msg, 2, ""),
    stdErr: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ExecuteFunctionResponse}
 */
proto.proto.api.v1.ExecuteFunctionResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ExecuteFunctionResponse;
  return proto.proto.api.v1.ExecuteFunctionResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ExecuteFunctionResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ExecuteFunctionResponse}
 */
proto.proto.api.v1.ExecuteFunctionResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_protobuf_struct_pb.Value;
      reader.readMessage(value,google_protobuf_struct_pb.Value.deserializeBinaryFromReader);
      msg.addResults(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setStdOut(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStdErr(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ExecuteFunctionResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ExecuteFunctionResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteFunctionResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_protobuf_struct_pb.Value.serializeBinaryToWriter
    );
  }
  f = message.getStdOut();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStdErr();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * repeated google.protobuf.Value results = 1;
 * @return {!Array<!proto.google.protobuf.Value>}
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.google.protobuf.Value>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_protobuf_struct_pb.Value, 1));
};


/**
 * @param {!Array<!proto.google.protobuf.Value>} value
 * @return {!proto.proto.api.v1.ExecuteFunctionResponse} returns this
*/
proto.proto.api.v1.ExecuteFunctionResponse.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.protobuf.Value=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.protobuf.Value}
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.protobuf.Value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.ExecuteFunctionResponse} returns this
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.clearResultsList = function() {
  return this.setResultsList([]);
};


/**
 * optional string std_out = 2;
 * @return {string}
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.getStdOut = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ExecuteFunctionResponse} returns this
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.setStdOut = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string std_err = 3;
 * @return {string}
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.getStdErr = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ExecuteFunctionResponse} returns this
 */
proto.proto.api.v1.ExecuteFunctionResponse.prototype.setStdErr = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ExecuteSourceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ExecuteSourceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ExecuteSourceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteSourceRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    source: jspb.Message.getFieldWithDefault(msg, 1, ""),
    engine: jspb.Message.getFieldWithDefault(msg, 2, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ExecuteSourceRequest}
 */
proto.proto.api.v1.ExecuteSourceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ExecuteSourceRequest;
  return proto.proto.api.v1.ExecuteSourceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ExecuteSourceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ExecuteSourceRequest}
 */
proto.proto.api.v1.ExecuteSourceRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setSource(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setEngine(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ExecuteSourceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ExecuteSourceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ExecuteSourceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteSourceRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getSource();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getEngine();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * optional string source = 1;
 * @return {string}
 */
proto.proto.api.v1.ExecuteSourceRequest.prototype.getSource = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ExecuteSourceRequest} returns this
 */
proto.proto.api.v1.ExecuteSourceRequest.prototype.setSource = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string engine = 2;
 * @return {string}
 */
proto.proto.api.v1.ExecuteSourceRequest.prototype.getEngine = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ExecuteSourceRequest} returns this
 */
proto.proto.api.v1.ExecuteSourceRequest.prototype.setEngine = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.ExecuteSourceResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ExecuteSourceResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ExecuteSourceResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteSourceResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    resultsList: jspb.Message.toObjectList(msg.getResultsList(),
    google_protobuf_struct_pb.Value.toObject, includeInstance),
    stdOut: jspb.Message.getFieldWithDefault(msg, 2, ""),
    stdErr: jspb.Message.getFieldWithDefault(msg, 3, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ExecuteSourceResponse}
 */
proto.proto.api.v1.ExecuteSourceResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ExecuteSourceResponse;
  return proto.proto.api.v1.ExecuteSourceResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ExecuteSourceResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ExecuteSourceResponse}
 */
proto.proto.api.v1.ExecuteSourceResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_protobuf_struct_pb.Value;
      reader.readMessage(value,google_protobuf_struct_pb.Value.deserializeBinaryFromReader);
      msg.addResults(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setStdOut(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setStdErr(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ExecuteSourceResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ExecuteSourceResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ExecuteSourceResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResultsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_protobuf_struct_pb.Value.serializeBinaryToWriter
    );
  }
  f = message.getStdOut();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getStdErr();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * repeated google.protobuf.Value results = 1;
 * @return {!Array<!proto.google.protobuf.Value>}
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.getResultsList = function() {
  return /** @type{!Array<!proto.google.protobuf.Value>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_protobuf_struct_pb.Value, 1));
};


/**
 * @param {!Array<!proto.google.protobuf.Value>} value
 * @return {!proto.proto.api.v1.ExecuteSourceResponse} returns this
*/
proto.proto.api.v1.ExecuteSourceResponse.prototype.setResultsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.protobuf.Value=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.protobuf.Value}
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.addResults = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.protobuf.Value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.ExecuteSourceResponse} returns this
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.clearResultsList = function() {
  return this.setResultsList([]);
};


/**
 * optional string std_out = 2;
 * @return {string}
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.getStdOut = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ExecuteSourceResponse} returns this
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.setStdOut = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string std_err = 3;
 * @return {string}
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.getStdErr = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ExecuteSourceResponse} returns this
 */
proto.proto.api.v1.ExecuteSourceResponse.prototype.setStdErr = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.MotorStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.MotorStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.MotorStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.MotorStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    on: jspb.Message.getBooleanFieldWithDefault(msg, 1, false),
    positionSupported: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
    position: jspb.Message.getFloatingPointFieldWithDefault(msg, 3, 0.0),
    pidConfig: (f = msg.getPidConfig()) && google_protobuf_struct_pb.Struct.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.MotorStatus}
 */
proto.proto.api.v1.MotorStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.MotorStatus;
  return proto.proto.api.v1.MotorStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.MotorStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.MotorStatus}
 */
proto.proto.api.v1.MotorStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setOn(value);
      break;
    case 2:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setPositionSupported(value);
      break;
    case 3:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setPosition(value);
      break;
    case 4:
      var value = new google_protobuf_struct_pb.Struct;
      reader.readMessage(value,google_protobuf_struct_pb.Struct.deserializeBinaryFromReader);
      msg.setPidConfig(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.MotorStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.MotorStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.MotorStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.MotorStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOn();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
  f = message.getPositionSupported();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getPosition();
  if (f !== 0.0) {
    writer.writeDouble(
      3,
      f
    );
  }
  f = message.getPidConfig();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      google_protobuf_struct_pb.Struct.serializeBinaryToWriter
    );
  }
};


/**
 * optional bool on = 1;
 * @return {boolean}
 */
proto.proto.api.v1.MotorStatus.prototype.getOn = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.proto.api.v1.MotorStatus} returns this
 */
proto.proto.api.v1.MotorStatus.prototype.setOn = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};


/**
 * optional bool position_supported = 2;
 * @return {boolean}
 */
proto.proto.api.v1.MotorStatus.prototype.getPositionSupported = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.proto.api.v1.MotorStatus} returns this
 */
proto.proto.api.v1.MotorStatus.prototype.setPositionSupported = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * optional double position = 3;
 * @return {number}
 */
proto.proto.api.v1.MotorStatus.prototype.getPosition = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 3, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.MotorStatus} returns this
 */
proto.proto.api.v1.MotorStatus.prototype.setPosition = function(value) {
  return jspb.Message.setProto3FloatField(this, 3, value);
};


/**
 * optional google.protobuf.Struct pid_config = 4;
 * @return {?proto.google.protobuf.Struct}
 */
proto.proto.api.v1.MotorStatus.prototype.getPidConfig = function() {
  return /** @type{?proto.google.protobuf.Struct} */ (
    jspb.Message.getWrapperField(this, google_protobuf_struct_pb.Struct, 4));
};


/**
 * @param {?proto.google.protobuf.Struct|undefined} value
 * @return {!proto.proto.api.v1.MotorStatus} returns this
*/
proto.proto.api.v1.MotorStatus.prototype.setPidConfig = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.MotorStatus} returns this
 */
proto.proto.api.v1.MotorStatus.prototype.clearPidConfig = function() {
  return this.setPidConfig(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.MotorStatus.prototype.hasPidConfig = function() {
  return jspb.Message.getField(this, 4) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ServoStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ServoStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ServoStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ServoStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    angle: jspb.Message.getFieldWithDefault(msg, 1, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ServoStatus}
 */
proto.proto.api.v1.ServoStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ServoStatus;
  return proto.proto.api.v1.ServoStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ServoStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ServoStatus}
 */
proto.proto.api.v1.ServoStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readUint32());
      msg.setAngle(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ServoStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ServoStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ServoStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ServoStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getAngle();
  if (f !== 0) {
    writer.writeUint32(
      1,
      f
    );
  }
};


/**
 * optional uint32 angle = 1;
 * @return {number}
 */
proto.proto.api.v1.ServoStatus.prototype.getAngle = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.ServoStatus} returns this
 */
proto.proto.api.v1.ServoStatus.prototype.setAngle = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ResourceRunCommandRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ResourceRunCommandRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ResourceRunCommandRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    resourceName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    commandName: jspb.Message.getFieldWithDefault(msg, 2, ""),
    args: (f = msg.getArgs()) && google_protobuf_struct_pb.Struct.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ResourceRunCommandRequest}
 */
proto.proto.api.v1.ResourceRunCommandRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ResourceRunCommandRequest;
  return proto.proto.api.v1.ResourceRunCommandRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ResourceRunCommandRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ResourceRunCommandRequest}
 */
proto.proto.api.v1.ResourceRunCommandRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setResourceName(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setCommandName(value);
      break;
    case 3:
      var value = new google_protobuf_struct_pb.Struct;
      reader.readMessage(value,google_protobuf_struct_pb.Struct.deserializeBinaryFromReader);
      msg.setArgs(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ResourceRunCommandRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ResourceRunCommandRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ResourceRunCommandRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResourceName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCommandName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getArgs();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      google_protobuf_struct_pb.Struct.serializeBinaryToWriter
    );
  }
};


/**
 * optional string resource_name = 1;
 * @return {string}
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.getResourceName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ResourceRunCommandRequest} returns this
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.setResourceName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional string command_name = 2;
 * @return {string}
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.getCommandName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ResourceRunCommandRequest} returns this
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.setCommandName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional google.protobuf.Struct args = 3;
 * @return {?proto.google.protobuf.Struct}
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.getArgs = function() {
  return /** @type{?proto.google.protobuf.Struct} */ (
    jspb.Message.getWrapperField(this, google_protobuf_struct_pb.Struct, 3));
};


/**
 * @param {?proto.google.protobuf.Struct|undefined} value
 * @return {!proto.proto.api.v1.ResourceRunCommandRequest} returns this
*/
proto.proto.api.v1.ResourceRunCommandRequest.prototype.setArgs = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.ResourceRunCommandRequest} returns this
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.clearArgs = function() {
  return this.setArgs(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.ResourceRunCommandRequest.prototype.hasArgs = function() {
  return jspb.Message.getField(this, 3) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ResourceRunCommandResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ResourceRunCommandResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ResourceRunCommandResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ResourceRunCommandResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    result: (f = msg.getResult()) && google_protobuf_struct_pb.Struct.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ResourceRunCommandResponse}
 */
proto.proto.api.v1.ResourceRunCommandResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ResourceRunCommandResponse;
  return proto.proto.api.v1.ResourceRunCommandResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ResourceRunCommandResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ResourceRunCommandResponse}
 */
proto.proto.api.v1.ResourceRunCommandResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_protobuf_struct_pb.Struct;
      reader.readMessage(value,google_protobuf_struct_pb.Struct.deserializeBinaryFromReader);
      msg.setResult(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ResourceRunCommandResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ResourceRunCommandResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ResourceRunCommandResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ResourceRunCommandResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getResult();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      google_protobuf_struct_pb.Struct.serializeBinaryToWriter
    );
  }
};


/**
 * optional google.protobuf.Struct result = 1;
 * @return {?proto.google.protobuf.Struct}
 */
proto.proto.api.v1.ResourceRunCommandResponse.prototype.getResult = function() {
  return /** @type{?proto.google.protobuf.Struct} */ (
    jspb.Message.getWrapperField(this, google_protobuf_struct_pb.Struct, 1));
};


/**
 * @param {?proto.google.protobuf.Struct|undefined} value
 * @return {!proto.proto.api.v1.ResourceRunCommandResponse} returns this
*/
proto.proto.api.v1.ResourceRunCommandResponse.prototype.setResult = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.ResourceRunCommandResponse} returns this
 */
proto.proto.api.v1.ResourceRunCommandResponse.prototype.clearResult = function() {
  return this.setResult(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.ResourceRunCommandResponse.prototype.hasResult = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceModeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceModeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceModeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceModeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceModeRequest}
 */
proto.proto.api.v1.NavigationServiceModeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceModeRequest;
  return proto.proto.api.v1.NavigationServiceModeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceModeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceModeRequest}
 */
proto.proto.api.v1.NavigationServiceModeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceModeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceModeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceModeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceModeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceModeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceModeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceModeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceModeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    mode: jspb.Message.getFieldWithDefault(msg, 1, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceModeResponse}
 */
proto.proto.api.v1.NavigationServiceModeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceModeResponse;
  return proto.proto.api.v1.NavigationServiceModeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceModeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceModeResponse}
 */
proto.proto.api.v1.NavigationServiceModeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.proto.api.v1.NavigationServiceMode} */ (reader.readEnum());
      msg.setMode(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceModeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceModeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceModeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceModeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
};


/**
 * optional NavigationServiceMode mode = 1;
 * @return {!proto.proto.api.v1.NavigationServiceMode}
 */
proto.proto.api.v1.NavigationServiceModeResponse.prototype.getMode = function() {
  return /** @type {!proto.proto.api.v1.NavigationServiceMode} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceMode} value
 * @return {!proto.proto.api.v1.NavigationServiceModeResponse} returns this
 */
proto.proto.api.v1.NavigationServiceModeResponse.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceSetModeRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    mode: jspb.Message.getFieldWithDefault(msg, 1, 0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceSetModeRequest}
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceSetModeRequest;
  return proto.proto.api.v1.NavigationServiceSetModeRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceSetModeRequest}
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {!proto.proto.api.v1.NavigationServiceMode} */ (reader.readEnum());
      msg.setMode(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceSetModeRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceSetModeRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMode();
  if (f !== 0.0) {
    writer.writeEnum(
      1,
      f
    );
  }
};


/**
 * optional NavigationServiceMode mode = 1;
 * @return {!proto.proto.api.v1.NavigationServiceMode}
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.prototype.getMode = function() {
  return /** @type {!proto.proto.api.v1.NavigationServiceMode} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceMode} value
 * @return {!proto.proto.api.v1.NavigationServiceSetModeRequest} returns this
 */
proto.proto.api.v1.NavigationServiceSetModeRequest.prototype.setMode = function(value) {
  return jspb.Message.setProto3EnumField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceSetModeResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceSetModeResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceSetModeResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceSetModeResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceSetModeResponse}
 */
proto.proto.api.v1.NavigationServiceSetModeResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceSetModeResponse;
  return proto.proto.api.v1.NavigationServiceSetModeResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceSetModeResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceSetModeResponse}
 */
proto.proto.api.v1.NavigationServiceSetModeResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceSetModeResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceSetModeResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceSetModeResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceSetModeResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceWaypoint.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceWaypoint.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceWaypoint} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceWaypoint.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, ""),
    location: (f = msg.getLocation()) && proto_api_common_v1_common_pb.GeoPoint.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceWaypoint}
 */
proto.proto.api.v1.NavigationServiceWaypoint.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceWaypoint;
  return proto.proto.api.v1.NavigationServiceWaypoint.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceWaypoint} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceWaypoint}
 */
proto.proto.api.v1.NavigationServiceWaypoint.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    case 2:
      var value = new proto_api_common_v1_common_pb.GeoPoint;
      reader.readMessage(value,proto_api_common_v1_common_pb.GeoPoint.deserializeBinaryFromReader);
      msg.setLocation(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceWaypoint.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceWaypoint.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceWaypoint} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceWaypoint.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getLocation();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto_api_common_v1_common_pb.GeoPoint.serializeBinaryToWriter
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.proto.api.v1.NavigationServiceWaypoint.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.NavigationServiceWaypoint} returns this
 */
proto.proto.api.v1.NavigationServiceWaypoint.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional proto.api.common.v1.GeoPoint location = 2;
 * @return {?proto.proto.api.common.v1.GeoPoint}
 */
proto.proto.api.v1.NavigationServiceWaypoint.prototype.getLocation = function() {
  return /** @type{?proto.proto.api.common.v1.GeoPoint} */ (
    jspb.Message.getWrapperField(this, proto_api_common_v1_common_pb.GeoPoint, 2));
};


/**
 * @param {?proto.proto.api.common.v1.GeoPoint|undefined} value
 * @return {!proto.proto.api.v1.NavigationServiceWaypoint} returns this
*/
proto.proto.api.v1.NavigationServiceWaypoint.prototype.setLocation = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.NavigationServiceWaypoint} returns this
 */
proto.proto.api.v1.NavigationServiceWaypoint.prototype.clearLocation = function() {
  return this.setLocation(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.NavigationServiceWaypoint.prototype.hasLocation = function() {
  return jspb.Message.getField(this, 2) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceLocationRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceLocationRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceLocationRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceLocationRequest}
 */
proto.proto.api.v1.NavigationServiceLocationRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceLocationRequest;
  return proto.proto.api.v1.NavigationServiceLocationRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceLocationRequest}
 */
proto.proto.api.v1.NavigationServiceLocationRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceLocationRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceLocationRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceLocationRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceLocationRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceLocationResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceLocationResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceLocationResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceLocationResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    location: (f = msg.getLocation()) && proto_api_common_v1_common_pb.GeoPoint.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceLocationResponse}
 */
proto.proto.api.v1.NavigationServiceLocationResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceLocationResponse;
  return proto.proto.api.v1.NavigationServiceLocationResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceLocationResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceLocationResponse}
 */
proto.proto.api.v1.NavigationServiceLocationResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto_api_common_v1_common_pb.GeoPoint;
      reader.readMessage(value,proto_api_common_v1_common_pb.GeoPoint.deserializeBinaryFromReader);
      msg.setLocation(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceLocationResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceLocationResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceLocationResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceLocationResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getLocation();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto_api_common_v1_common_pb.GeoPoint.serializeBinaryToWriter
    );
  }
};


/**
 * optional proto.api.common.v1.GeoPoint location = 1;
 * @return {?proto.proto.api.common.v1.GeoPoint}
 */
proto.proto.api.v1.NavigationServiceLocationResponse.prototype.getLocation = function() {
  return /** @type{?proto.proto.api.common.v1.GeoPoint} */ (
    jspb.Message.getWrapperField(this, proto_api_common_v1_common_pb.GeoPoint, 1));
};


/**
 * @param {?proto.proto.api.common.v1.GeoPoint|undefined} value
 * @return {!proto.proto.api.v1.NavigationServiceLocationResponse} returns this
*/
proto.proto.api.v1.NavigationServiceLocationResponse.prototype.setLocation = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.NavigationServiceLocationResponse} returns this
 */
proto.proto.api.v1.NavigationServiceLocationResponse.prototype.clearLocation = function() {
  return this.setLocation(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.NavigationServiceLocationResponse.prototype.hasLocation = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceWaypointsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceWaypointsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceWaypointsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceWaypointsRequest}
 */
proto.proto.api.v1.NavigationServiceWaypointsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceWaypointsRequest;
  return proto.proto.api.v1.NavigationServiceWaypointsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceWaypointsRequest}
 */
proto.proto.api.v1.NavigationServiceWaypointsRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceWaypointsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceWaypointsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceWaypointsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceWaypointsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    waypointsList: jspb.Message.toObjectList(msg.getWaypointsList(),
    proto.proto.api.v1.NavigationServiceWaypoint.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceWaypointsResponse}
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceWaypointsResponse;
  return proto.proto.api.v1.NavigationServiceWaypointsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceWaypointsResponse}
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.proto.api.v1.NavigationServiceWaypoint;
      reader.readMessage(value,proto.proto.api.v1.NavigationServiceWaypoint.deserializeBinaryFromReader);
      msg.addWaypoints(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceWaypointsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceWaypointsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getWaypointsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.proto.api.v1.NavigationServiceWaypoint.serializeBinaryToWriter
    );
  }
};


/**
 * repeated NavigationServiceWaypoint waypoints = 1;
 * @return {!Array<!proto.proto.api.v1.NavigationServiceWaypoint>}
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.prototype.getWaypointsList = function() {
  return /** @type{!Array<!proto.proto.api.v1.NavigationServiceWaypoint>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.proto.api.v1.NavigationServiceWaypoint, 1));
};


/**
 * @param {!Array<!proto.proto.api.v1.NavigationServiceWaypoint>} value
 * @return {!proto.proto.api.v1.NavigationServiceWaypointsResponse} returns this
*/
proto.proto.api.v1.NavigationServiceWaypointsResponse.prototype.setWaypointsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.proto.api.v1.NavigationServiceWaypoint=} opt_value
 * @param {number=} opt_index
 * @return {!proto.proto.api.v1.NavigationServiceWaypoint}
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.prototype.addWaypoints = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.proto.api.v1.NavigationServiceWaypoint, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.NavigationServiceWaypointsResponse} returns this
 */
proto.proto.api.v1.NavigationServiceWaypointsResponse.prototype.clearWaypointsList = function() {
  return this.setWaypointsList([]);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceAddWaypointRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    location: (f = msg.getLocation()) && proto_api_common_v1_common_pb.GeoPoint.toObject(includeInstance, f)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceAddWaypointRequest}
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceAddWaypointRequest;
  return proto.proto.api.v1.NavigationServiceAddWaypointRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceAddWaypointRequest}
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto_api_common_v1_common_pb.GeoPoint;
      reader.readMessage(value,proto_api_common_v1_common_pb.GeoPoint.deserializeBinaryFromReader);
      msg.setLocation(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceAddWaypointRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getLocation();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      proto_api_common_v1_common_pb.GeoPoint.serializeBinaryToWriter
    );
  }
};


/**
 * optional proto.api.common.v1.GeoPoint location = 1;
 * @return {?proto.proto.api.common.v1.GeoPoint}
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.prototype.getLocation = function() {
  return /** @type{?proto.proto.api.common.v1.GeoPoint} */ (
    jspb.Message.getWrapperField(this, proto_api_common_v1_common_pb.GeoPoint, 1));
};


/**
 * @param {?proto.proto.api.common.v1.GeoPoint|undefined} value
 * @return {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} returns this
*/
proto.proto.api.v1.NavigationServiceAddWaypointRequest.prototype.setLocation = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.NavigationServiceAddWaypointRequest} returns this
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.prototype.clearLocation = function() {
  return this.setLocation(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.NavigationServiceAddWaypointRequest.prototype.hasLocation = function() {
  return jspb.Message.getField(this, 1) != null;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceAddWaypointResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceAddWaypointResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceAddWaypointResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceAddWaypointResponse}
 */
proto.proto.api.v1.NavigationServiceAddWaypointResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceAddWaypointResponse;
  return proto.proto.api.v1.NavigationServiceAddWaypointResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceAddWaypointResponse}
 */
proto.proto.api.v1.NavigationServiceAddWaypointResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceAddWaypointResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceAddWaypointResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceAddWaypointResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceAddWaypointResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceRemoveWaypointRequest;
  return proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setId(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string id = 1;
 * @return {string}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.prototype.getId = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.NavigationServiceRemoveWaypointRequest} returns this
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointRequest.prototype.setId = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.toObject = function(includeInstance, msg) {
  var f, obj = {

  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.NavigationServiceRemoveWaypointResponse;
  return proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.NavigationServiceRemoveWaypointResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.NavigationServiceRemoveWaypointResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    cameraName: jspb.Message.getFieldWithDefault(msg, 1, ""),
    cameraPoint: (f = msg.getCameraPoint()) && proto.proto.api.v1.Vector3.toObject(includeInstance, f),
    gripperName: jspb.Message.getFieldWithDefault(msg, 3, ""),
    armName: jspb.Message.getFieldWithDefault(msg, 4, "")
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest;
  return proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {string} */ (reader.readString());
      msg.setCameraName(value);
      break;
    case 2:
      var value = new proto.proto.api.v1.Vector3;
      reader.readMessage(value,proto.proto.api.v1.Vector3.deserializeBinaryFromReader);
      msg.setCameraPoint(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setGripperName(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setArmName(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getCameraName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getCameraPoint();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.proto.api.v1.Vector3.serializeBinaryToWriter
    );
  }
  f = message.getGripperName();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getArmName();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string camera_name = 1;
 * @return {string}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.getCameraName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} returns this
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.setCameraName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional Vector3 camera_point = 2;
 * @return {?proto.proto.api.v1.Vector3}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.getCameraPoint = function() {
  return /** @type{?proto.proto.api.v1.Vector3} */ (
    jspb.Message.getWrapperField(this, proto.proto.api.v1.Vector3, 2));
};


/**
 * @param {?proto.proto.api.v1.Vector3|undefined} value
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} returns this
*/
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.setCameraPoint = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} returns this
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.clearCameraPoint = function() {
  return this.setCameraPoint(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.hasCameraPoint = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional string gripper_name = 3;
 * @return {string}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.getGripperName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} returns this
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.setGripperName = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string arm_name = 4;
 * @return {string}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.getArmName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest} returns this
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabRequest.prototype.setArmName = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    hasGrabbed: jspb.Message.getBooleanFieldWithDefault(msg, 1, false)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse;
  return proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setHasGrabbed(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getHasGrabbed();
  if (f) {
    writer.writeBool(
      1,
      f
    );
  }
};


/**
 * optional bool has_grabbed = 1;
 * @return {boolean}
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.prototype.getHasGrabbed = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 1, false));
};


/**
 * @param {boolean} value
 * @return {!proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse} returns this
 */
proto.proto.api.v1.ObjectManipulationServiceDoGrabResponse.prototype.setHasGrabbed = function(value) {
  return jspb.Message.setProto3BooleanField(this, 1, value);
};





if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.InputControllerEvent.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.InputControllerEvent.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.InputControllerEvent} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.InputControllerEvent.toObject = function(includeInstance, msg) {
  var f, obj = {
    time: (f = msg.getTime()) && google_protobuf_timestamp_pb.Timestamp.toObject(includeInstance, f),
    event: jspb.Message.getFieldWithDefault(msg, 2, ""),
    control: jspb.Message.getFieldWithDefault(msg, 3, ""),
    value: jspb.Message.getFloatingPointFieldWithDefault(msg, 4, 0.0)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.InputControllerEvent}
 */
proto.proto.api.v1.InputControllerEvent.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.InputControllerEvent;
  return proto.proto.api.v1.InputControllerEvent.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.InputControllerEvent} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.InputControllerEvent}
 */
proto.proto.api.v1.InputControllerEvent.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_protobuf_timestamp_pb.Timestamp;
      reader.readMessage(value,google_protobuf_timestamp_pb.Timestamp.deserializeBinaryFromReader);
      msg.setTime(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setEvent(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setControl(value);
      break;
    case 4:
      var value = /** @type {number} */ (reader.readDouble());
      msg.setValue(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.InputControllerEvent.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.InputControllerEvent.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.InputControllerEvent} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.InputControllerEvent.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getTime();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      google_protobuf_timestamp_pb.Timestamp.serializeBinaryToWriter
    );
  }
  f = message.getEvent();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getControl();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getValue();
  if (f !== 0.0) {
    writer.writeDouble(
      4,
      f
    );
  }
};


/**
 * optional google.protobuf.Timestamp time = 1;
 * @return {?proto.google.protobuf.Timestamp}
 */
proto.proto.api.v1.InputControllerEvent.prototype.getTime = function() {
  return /** @type{?proto.google.protobuf.Timestamp} */ (
    jspb.Message.getWrapperField(this, google_protobuf_timestamp_pb.Timestamp, 1));
};


/**
 * @param {?proto.google.protobuf.Timestamp|undefined} value
 * @return {!proto.proto.api.v1.InputControllerEvent} returns this
*/
proto.proto.api.v1.InputControllerEvent.prototype.setTime = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.proto.api.v1.InputControllerEvent} returns this
 */
proto.proto.api.v1.InputControllerEvent.prototype.clearTime = function() {
  return this.setTime(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.proto.api.v1.InputControllerEvent.prototype.hasTime = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional string event = 2;
 * @return {string}
 */
proto.proto.api.v1.InputControllerEvent.prototype.getEvent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.InputControllerEvent} returns this
 */
proto.proto.api.v1.InputControllerEvent.prototype.setEvent = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string control = 3;
 * @return {string}
 */
proto.proto.api.v1.InputControllerEvent.prototype.getControl = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.proto.api.v1.InputControllerEvent} returns this
 */
proto.proto.api.v1.InputControllerEvent.prototype.setControl = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional double value = 4;
 * @return {number}
 */
proto.proto.api.v1.InputControllerEvent.prototype.getValue = function() {
  return /** @type {number} */ (jspb.Message.getFloatingPointFieldWithDefault(this, 4, 0.0));
};


/**
 * @param {number} value
 * @return {!proto.proto.api.v1.InputControllerEvent} returns this
 */
proto.proto.api.v1.InputControllerEvent.prototype.setValue = function(value) {
  return jspb.Message.setProto3FloatField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.proto.api.v1.InputControllerStatus.repeatedFields_ = [1];



if (jspb.Message.GENERATE_TO_OBJECT) {
/**
 * Creates an object representation of this proto.
 * Field names that are reserved in JavaScript and will be renamed to pb_name.
 * Optional fields that are not set will be set to undefined.
 * To access a reserved field use, foo.pb_<name>, eg, foo.pb_default.
 * For the list of reserved names please see:
 *     net/proto2/compiler/js/internal/generator.cc#kKeyword.
 * @param {boolean=} opt_includeInstance Deprecated. whether to include the
 *     JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @return {!Object}
 */
proto.proto.api.v1.InputControllerStatus.prototype.toObject = function(opt_includeInstance) {
  return proto.proto.api.v1.InputControllerStatus.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.proto.api.v1.InputControllerStatus} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.InputControllerStatus.toObject = function(includeInstance, msg) {
  var f, obj = {
    eventsList: jspb.Message.toObjectList(msg.getEventsList(),
    proto.proto.api.v1.InputControllerEvent.toObject, includeInstance)
  };

  if (includeInstance) {
    obj.$jspbMessageInstance = msg;
  }
  return obj;
};
}


/**
 * Deserializes binary data (in protobuf wire format).
 * @param {jspb.ByteSource} bytes The bytes to deserialize.
 * @return {!proto.proto.api.v1.InputControllerStatus}
 */
proto.proto.api.v1.InputControllerStatus.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.proto.api.v1.InputControllerStatus;
  return proto.proto.api.v1.InputControllerStatus.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.proto.api.v1.InputControllerStatus} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.proto.api.v1.InputControllerStatus}
 */
proto.proto.api.v1.InputControllerStatus.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.proto.api.v1.InputControllerEvent;
      reader.readMessage(value,proto.proto.api.v1.InputControllerEvent.deserializeBinaryFromReader);
      msg.addEvents(value);
      break;
    default:
      reader.skipField();
      break;
    }
  }
  return msg;
};


/**
 * Serializes the message to binary data (in protobuf wire format).
 * @return {!Uint8Array}
 */
proto.proto.api.v1.InputControllerStatus.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.proto.api.v1.InputControllerStatus.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.proto.api.v1.InputControllerStatus} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.proto.api.v1.InputControllerStatus.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getEventsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.proto.api.v1.InputControllerEvent.serializeBinaryToWriter
    );
  }
};


/**
 * repeated InputControllerEvent events = 1;
 * @return {!Array<!proto.proto.api.v1.InputControllerEvent>}
 */
proto.proto.api.v1.InputControllerStatus.prototype.getEventsList = function() {
  return /** @type{!Array<!proto.proto.api.v1.InputControllerEvent>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.proto.api.v1.InputControllerEvent, 1));
};


/**
 * @param {!Array<!proto.proto.api.v1.InputControllerEvent>} value
 * @return {!proto.proto.api.v1.InputControllerStatus} returns this
*/
proto.proto.api.v1.InputControllerStatus.prototype.setEventsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.proto.api.v1.InputControllerEvent=} opt_value
 * @param {number=} opt_index
 * @return {!proto.proto.api.v1.InputControllerEvent}
 */
proto.proto.api.v1.InputControllerStatus.prototype.addEvents = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.proto.api.v1.InputControllerEvent, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.proto.api.v1.InputControllerStatus} returns this
 */
proto.proto.api.v1.InputControllerStatus.prototype.clearEventsList = function() {
  return this.setEventsList([]);
};


/**
 * @enum {number}
 */
proto.proto.api.v1.NavigationServiceMode = {
  NAVIGATION_SERVICE_MODE_UNSPECIFIED: 0,
  NAVIGATION_SERVICE_MODE_MANUAL: 1,
  NAVIGATION_SERVICE_MODE_WAYPOINT: 2
};

goog.object.extend(exports, proto.proto.api.v1);
