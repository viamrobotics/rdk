// source: google/api/serviceusage/v1beta1/serviceusage.proto
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
var global = Function('return this')();

var google_api_annotations_pb = require('../../../../google/api/annotations_pb.js');
goog.object.extend(proto, google_api_annotations_pb);
var google_api_client_pb = require('../../../../google/api/client_pb.js');
goog.object.extend(proto, google_api_client_pb);
var google_api_field_behavior_pb = require('../../../../google/api/field_behavior_pb.js');
goog.object.extend(proto, google_api_field_behavior_pb);
var google_api_serviceusage_v1beta1_resources_pb = require('../../../../google/api/serviceusage/v1beta1/resources_pb.js');
goog.object.extend(proto, google_api_serviceusage_v1beta1_resources_pb);
var google_longrunning_operations_pb = require('../../../../google/longrunning/operations_pb.js');
goog.object.extend(proto, google_longrunning_operations_pb);
var google_protobuf_field_mask_pb = require('google-protobuf/google/protobuf/field_mask_pb.js');
goog.object.extend(proto, google_protobuf_field_mask_pb);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.DisableServiceRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.EnableServiceRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.IdentityState', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.GetServiceRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.SourceCase', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.SourceCase', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListServicesRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.ListServicesResponse', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata', null, global);
goog.exportSymbol('proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest', null, global);
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
proto.google.api.serviceusage.v1beta1.EnableServiceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.EnableServiceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.EnableServiceRequest.displayName = 'proto.google.api.serviceusage.v1beta1.EnableServiceRequest';
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
proto.google.api.serviceusage.v1beta1.DisableServiceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.DisableServiceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.DisableServiceRequest.displayName = 'proto.google.api.serviceusage.v1beta1.DisableServiceRequest';
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
proto.google.api.serviceusage.v1beta1.GetServiceRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.GetServiceRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.GetServiceRequest.displayName = 'proto.google.api.serviceusage.v1beta1.GetServiceRequest';
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
proto.google.api.serviceusage.v1beta1.ListServicesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListServicesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListServicesRequest.displayName = 'proto.google.api.serviceusage.v1beta1.ListServicesRequest';
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
proto.google.api.serviceusage.v1beta1.ListServicesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ListServicesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListServicesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListServicesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.ListServicesResponse';
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
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.displayName = 'proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest';
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
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.displayName = 'proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest';
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
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.displayName = 'proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse';
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
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.displayName = 'proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest';
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
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.displayName = 'proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest';
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
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.displayName = 'proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest';
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
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.displayName = 'proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest';
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
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.displayName = 'proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest';
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
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.displayName = 'proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest';
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
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse';
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
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse';
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.repeatedFields_, proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.oneofGroups_);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.displayName = 'proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest';
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse';
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.displayName = 'proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata';
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
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.displayName = 'proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest';
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
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.displayName = 'proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest';
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
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.displayName = 'proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest';
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
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.displayName = 'proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest';
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
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse';
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
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse';
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.repeatedFields_, proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.oneofGroups_);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.displayName = 'proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest';
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse';
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.displayName = 'proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata';
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
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.repeatedFields_, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.displayName = 'proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse';
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
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.displayName = 'proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata';
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
proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.displayName = 'proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata';
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
proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.displayName = 'proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata';
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
proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.displayName = 'proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata';
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
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.displayName = 'proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest';
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
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.displayName = 'proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse';
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
proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.displayName = 'proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata';
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
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.EnableServiceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest}
 */
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.EnableServiceRequest;
  return proto.google.api.serviceusage.v1beta1.EnableServiceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest}
 */
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.EnableServiceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.EnableServiceRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.EnableServiceRequest.prototype.setName = function(value) {
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
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.DisableServiceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest}
 */
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.DisableServiceRequest;
  return proto.google.api.serviceusage.v1beta1.DisableServiceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest}
 */
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.DisableServiceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.DisableServiceRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DisableServiceRequest.prototype.setName = function(value) {
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
proto.google.api.serviceusage.v1beta1.GetServiceRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.GetServiceRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetServiceRequest.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceRequest}
 */
proto.google.api.serviceusage.v1beta1.GetServiceRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.GetServiceRequest;
  return proto.google.api.serviceusage.v1beta1.GetServiceRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceRequest}
 */
proto.google.api.serviceusage.v1beta1.GetServiceRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.GetServiceRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.GetServiceRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetServiceRequest.serializeBinaryToWriter = function(message, writer) {
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
proto.google.api.serviceusage.v1beta1.GetServiceRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.GetServiceRequest.prototype.setName = function(value) {
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
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListServicesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    pageSize: jspb.Message.getFieldWithDefault(msg, 2, 0),
    pageToken: jspb.Message.getFieldWithDefault(msg, 3, ""),
    filter: jspb.Message.getFieldWithDefault(msg, 4, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesRequest}
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListServicesRequest;
  return proto.google.api.serviceusage.v1beta1.ListServicesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesRequest}
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPageSize(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
      break;
    case 4:
      var value = /** @type {string} */ (reader.readString());
      msg.setFilter(value);
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
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListServicesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPageSize();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getFilter();
  if (f.length > 0) {
    writer.writeString(
      4,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 page_size = 2;
 * @return {number}
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.getPageSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.setPageSize = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string page_token = 3;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional string filter = 4;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.getFilter = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 4, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListServicesRequest.prototype.setFilter = function(value) {
  return jspb.Message.setProto3StringField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListServicesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    servicesList: jspb.Message.toObjectList(msg.getServicesList(),
    google_api_serviceusage_v1beta1_resources_pb.Service.toObject, includeInstance),
    nextPageToken: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesResponse}
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListServicesResponse;
  return proto.google.api.serviceusage.v1beta1.ListServicesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesResponse}
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.Service;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.Service.deserializeBinaryFromReader);
      msg.addServices(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
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
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListServicesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListServicesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getServicesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.Service.serializeBinaryToWriter
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated Service services = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.Service>}
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.getServicesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.Service>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.Service, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.Service>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.setServicesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.Service=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.Service}
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.addServices = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.Service, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.clearServicesList = function() {
  return this.setServicesList([]);
};


/**
 * optional string next_page_token = 2;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListServicesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListServicesResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.repeatedFields_ = [2];



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
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    serviceIdsList: (f = jspb.Message.getRepeatedField(msg, 2)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest}
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest;
  return proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest}
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {string} */ (reader.readString());
      msg.addServiceIds(value);
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
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getServiceIdsList();
  if (f.length > 0) {
    writer.writeRepeatedString(
      2,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * repeated string service_ids = 2;
 * @return {!Array<string>}
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.getServiceIdsList = function() {
  return /** @type {!Array<string>} */ (jspb.Message.getRepeatedField(this, 2));
};


/**
 * @param {!Array<string>} value
 * @return {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.setServiceIdsList = function(value) {
  return jspb.Message.setField(this, 2, value || []);
};


/**
 * @param {string} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.addServiceIds = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 2, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.BatchEnableServicesRequest.prototype.clearServiceIdsList = function() {
  return this.setServiceIdsList([]);
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
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    pageSize: jspb.Message.getFieldWithDefault(msg, 2, 0),
    pageToken: jspb.Message.getFieldWithDefault(msg, 3, ""),
    view: jspb.Message.getFieldWithDefault(msg, 4, 0)
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest;
  return proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPageSize(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
      break;
    case 4:
      var value = /** @type {!proto.google.api.serviceusage.v1beta1.QuotaView} */ (reader.readEnum());
      msg.setView(value);
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
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPageSize();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getView();
  if (f !== 0.0) {
    writer.writeEnum(
      4,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 page_size = 2;
 * @return {number}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.getPageSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.setPageSize = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string page_token = 3;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional QuotaView view = 4;
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaView}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.getView = function() {
  return /** @type {!proto.google.api.serviceusage.v1beta1.QuotaView} */ (jspb.Message.getFieldWithDefault(this, 4, 0));
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaView} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsRequest.prototype.setView = function(value) {
  return jspb.Message.setProto3EnumField(this, 4, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    metricsList: jspb.Message.toObjectList(msg.getMetricsList(),
    google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric.toObject, includeInstance),
    nextPageToken: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse;
  return proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric.deserializeBinaryFromReader);
      msg.addMetrics(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
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
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getMetricsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric.serializeBinaryToWriter
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated ConsumerQuotaMetric metrics = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric>}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.getMetricsList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.ConsumerQuotaMetric, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.setMetricsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.addMetrics = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.ConsumerQuotaMetric, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.clearMetricsList = function() {
  return this.setMetricsList([]);
};


/**
 * optional string next_page_token = 2;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerQuotaMetricsResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
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
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    view: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest;
  return proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.google.api.serviceusage.v1beta1.QuotaView} */ (reader.readEnum());
      msg.setView(value);
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
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getView();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional QuotaView view = 2;
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaView}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.prototype.getView = function() {
  return /** @type {!proto.google.api.serviceusage.v1beta1.QuotaView} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaView} value
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaMetricRequest.prototype.setView = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
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
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    view: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest;
  return proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {!proto.google.api.serviceusage.v1beta1.QuotaView} */ (reader.readEnum());
      msg.setView(value);
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
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getView();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional QuotaView view = 2;
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaView}
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.prototype.getView = function() {
  return /** @type {!proto.google.api.serviceusage.v1beta1.QuotaView} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaView} value
 * @return {!proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.GetConsumerQuotaLimitRequest.prototype.setView = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.repeatedFields_ = [4];



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
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    override: (f = msg.getOverride()) && google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject(includeInstance, f),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest;
  return proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.setOverride(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 4:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOverride();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      4,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional QuotaOverride override = 2;
 * @return {?proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.getOverride = function() {
  return /** @type{?proto.google.api.serviceusage.v1beta1.QuotaOverride} */ (
    jspb.Message.getWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 2));
};


/**
 * @param {?proto.google.api.serviceusage.v1beta1.QuotaOverride|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.setOverride = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.clearOverride = function() {
  return this.setOverride(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.hasOverride = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool force = 3;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated QuotaSafetyCheck force_only = 4;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateAdminOverrideRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.repeatedFields_ = [5];



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
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    override: (f = msg.getOverride()) && google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject(includeInstance, f),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    updateMask: (f = msg.getUpdateMask()) && google_protobuf_field_mask_pb.FieldMask.toObject(includeInstance, f),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest;
  return proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.setOverride(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 4:
      var value = new google_protobuf_field_mask_pb.FieldMask;
      reader.readMessage(value,google_protobuf_field_mask_pb.FieldMask.deserializeBinaryFromReader);
      msg.setUpdateMask(value);
      break;
    case 5:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOverride();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getUpdateMask();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      google_protobuf_field_mask_pb.FieldMask.serializeBinaryToWriter
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      5,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional QuotaOverride override = 2;
 * @return {?proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.getOverride = function() {
  return /** @type{?proto.google.api.serviceusage.v1beta1.QuotaOverride} */ (
    jspb.Message.getWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 2));
};


/**
 * @param {?proto.google.api.serviceusage.v1beta1.QuotaOverride|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.setOverride = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.clearOverride = function() {
  return this.setOverride(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.hasOverride = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool force = 3;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional google.protobuf.FieldMask update_mask = 4;
 * @return {?proto.google.protobuf.FieldMask}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.getUpdateMask = function() {
  return /** @type{?proto.google.protobuf.FieldMask} */ (
    jspb.Message.getWrapperField(this, google_protobuf_field_mask_pb.FieldMask, 4));
};


/**
 * @param {?proto.google.protobuf.FieldMask|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.setUpdateMask = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.clearUpdateMask = function() {
  return this.setUpdateMask(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.hasUpdateMask = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * repeated QuotaSafetyCheck force_only = 5;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminOverrideRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.repeatedFields_ = [3];



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
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest;
  return proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 3:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      3,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool force = 2;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * repeated QuotaSafetyCheck force_only = 3;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminOverrideRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
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
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    pageSize: jspb.Message.getFieldWithDefault(msg, 2, 0),
    pageToken: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest;
  return proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPageSize(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
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
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPageSize();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 page_size = 2;
 * @return {number}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.getPageSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.setPageSize = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string page_token = 3;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    overridesList: jspb.Message.toObjectList(msg.getOverridesList(),
    google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject, includeInstance),
    nextPageToken: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse;
  return proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.addOverrides(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
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
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOverridesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated QuotaOverride overrides = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.getOverridesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.setOverridesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaOverride=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.addOverrides = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.QuotaOverride, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.clearOverridesList = function() {
  return this.setOverridesList([]);
};


/**
 * optional string next_page_token = 2;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListAdminOverridesResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    overridesList: jspb.Message.toObjectList(msg.getOverridesList(),
    google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject, includeInstance)
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
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse;
  return proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.addOverrides(value);
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
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOverridesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
};


/**
 * repeated QuotaOverride overrides = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.prototype.getOverridesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} value
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.prototype.setOverridesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaOverride=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.prototype.addOverrides = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.QuotaOverride, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.BatchCreateAdminOverridesResponse.prototype.clearOverridesList = function() {
  return this.setOverridesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.repeatedFields_ = [4];

/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.oneofGroups_ = [[2]];

/**
 * @enum {number}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.SourceCase = {
  SOURCE_NOT_SET: 0,
  INLINE_SOURCE: 2
};

/**
 * @return {proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.SourceCase}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.getSourceCase = function() {
  return /** @type {proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.SourceCase} */(jspb.Message.computeOneofCase(this, proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.oneofGroups_[0]));
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    inlineSource: (f = msg.getInlineSource()) && google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.toObject(includeInstance, f),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest;
  return proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.deserializeBinaryFromReader);
      msg.setInlineSource(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 4:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getInlineSource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.serializeBinaryToWriter
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      4,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional OverrideInlineSource inline_source = 2;
 * @return {?proto.google.api.serviceusage.v1beta1.OverrideInlineSource}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.getInlineSource = function() {
  return /** @type{?proto.google.api.serviceusage.v1beta1.OverrideInlineSource} */ (
    jspb.Message.getWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource, 2));
};


/**
 * @param {?proto.google.api.serviceusage.v1beta1.OverrideInlineSource|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.setInlineSource = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.clearInlineSource = function() {
  return this.setInlineSource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.hasInlineSource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool force = 3;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated QuotaSafetyCheck force_only = 4;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    overridesList: jspb.Message.toObjectList(msg.getOverridesList(),
    google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject, includeInstance)
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse;
  return proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.addOverrides(value);
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOverridesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
};


/**
 * repeated QuotaOverride overrides = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.prototype.getOverridesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.prototype.setOverridesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaOverride=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.prototype.addOverrides = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.QuotaOverride, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesResponse.prototype.clearOverridesList = function() {
  return this.setOverridesList([]);
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata;
  return proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminOverridesMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.repeatedFields_ = [4];



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
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    override: (f = msg.getOverride()) && google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject(includeInstance, f),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest;
  return proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.setOverride(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 4:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOverride();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      4,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional QuotaOverride override = 2;
 * @return {?proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.getOverride = function() {
  return /** @type{?proto.google.api.serviceusage.v1beta1.QuotaOverride} */ (
    jspb.Message.getWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 2));
};


/**
 * @param {?proto.google.api.serviceusage.v1beta1.QuotaOverride|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.setOverride = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.clearOverride = function() {
  return this.setOverride(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.hasOverride = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool force = 3;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated QuotaSafetyCheck force_only = 4;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.CreateConsumerOverrideRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.repeatedFields_ = [5];



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
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    override: (f = msg.getOverride()) && google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject(includeInstance, f),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    updateMask: (f = msg.getUpdateMask()) && google_protobuf_field_mask_pb.FieldMask.toObject(includeInstance, f),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 5)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest;
  return proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.setOverride(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 4:
      var value = new google_protobuf_field_mask_pb.FieldMask;
      reader.readMessage(value,google_protobuf_field_mask_pb.FieldMask.deserializeBinaryFromReader);
      msg.setUpdateMask(value);
      break;
    case 5:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getOverride();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getUpdateMask();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      google_protobuf_field_mask_pb.FieldMask.serializeBinaryToWriter
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      5,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional QuotaOverride override = 2;
 * @return {?proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.getOverride = function() {
  return /** @type{?proto.google.api.serviceusage.v1beta1.QuotaOverride} */ (
    jspb.Message.getWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 2));
};


/**
 * @param {?proto.google.api.serviceusage.v1beta1.QuotaOverride|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.setOverride = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.clearOverride = function() {
  return this.setOverride(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.hasOverride = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool force = 3;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * optional google.protobuf.FieldMask update_mask = 4;
 * @return {?proto.google.protobuf.FieldMask}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.getUpdateMask = function() {
  return /** @type{?proto.google.protobuf.FieldMask} */ (
    jspb.Message.getWrapperField(this, google_protobuf_field_mask_pb.FieldMask, 4));
};


/**
 * @param {?proto.google.protobuf.FieldMask|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.setUpdateMask = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.clearUpdateMask = function() {
  return this.setUpdateMask(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.hasUpdateMask = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * repeated QuotaSafetyCheck force_only = 5;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 5));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 5, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 5, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.UpdateConsumerOverrideRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.repeatedFields_ = [3];



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
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    name: jspb.Message.getFieldWithDefault(msg, 1, ""),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 2, false),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 3)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest;
  return proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest}
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 3:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      2,
      f
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      3,
      f
    );
  }
};


/**
 * optional string name = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional bool force = 2;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 2, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 2, value);
};


/**
 * repeated QuotaSafetyCheck force_only = 3;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 3));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 3, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 3, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.DeleteConsumerOverrideRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
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
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    pageSize: jspb.Message.getFieldWithDefault(msg, 2, 0),
    pageToken: jspb.Message.getFieldWithDefault(msg, 3, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest;
  return proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = /** @type {number} */ (reader.readInt32());
      msg.setPageSize(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setPageToken(value);
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
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getPageSize();
  if (f !== 0) {
    writer.writeInt32(
      2,
      f
    );
  }
  f = message.getPageToken();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional int32 page_size = 2;
 * @return {number}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.getPageSize = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {number} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.setPageSize = function(value) {
  return jspb.Message.setProto3IntField(this, 2, value);
};


/**
 * optional string page_token = 3;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.getPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesRequest.prototype.setPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    overridesList: jspb.Message.toObjectList(msg.getOverridesList(),
    google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject, includeInstance),
    nextPageToken: jspb.Message.getFieldWithDefault(msg, 2, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse;
  return proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.addOverrides(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setNextPageToken(value);
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
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOverridesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
  f = message.getNextPageToken();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
};


/**
 * repeated QuotaOverride overrides = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.getOverridesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.setOverridesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaOverride=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.addOverrides = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.QuotaOverride, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.clearOverridesList = function() {
  return this.setOverridesList([]);
};


/**
 * optional string next_page_token = 2;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.getNextPageToken = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ListConsumerOverridesResponse.prototype.setNextPageToken = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    overridesList: jspb.Message.toObjectList(msg.getOverridesList(),
    google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject, includeInstance)
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
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse;
  return proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.addOverrides(value);
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
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOverridesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
};


/**
 * repeated QuotaOverride overrides = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.prototype.getOverridesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} value
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.prototype.setOverridesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaOverride=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.prototype.addOverrides = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.QuotaOverride, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.BatchCreateConsumerOverridesResponse.prototype.clearOverridesList = function() {
  return this.setOverridesList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.repeatedFields_ = [4];

/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.oneofGroups_ = [[2]];

/**
 * @enum {number}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.SourceCase = {
  SOURCE_NOT_SET: 0,
  INLINE_SOURCE: 2
};

/**
 * @return {proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.SourceCase}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.getSourceCase = function() {
  return /** @type {proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.SourceCase} */(jspb.Message.computeOneofCase(this, proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.oneofGroups_[0]));
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, ""),
    inlineSource: (f = msg.getInlineSource()) && google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.toObject(includeInstance, f),
    force: jspb.Message.getBooleanFieldWithDefault(msg, 3, false),
    forceOnlyList: (f = jspb.Message.getRepeatedField(msg, 4)) == null ? undefined : f
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest;
  return proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.deserializeBinaryFromReader = function(msg, reader) {
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
      var value = new google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.deserializeBinaryFromReader);
      msg.setInlineSource(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setForce(value);
      break;
    case 4:
      var values = /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (reader.isDelimited() ? reader.readPackedEnum() : [reader.readEnum()]);
      for (var i = 0; i < values.length; i++) {
        msg.addForceOnly(values[i]);
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
  f = message.getInlineSource();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource.serializeBinaryToWriter
    );
  }
  f = message.getForce();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
  f = message.getForceOnlyList();
  if (f.length > 0) {
    writer.writePackedEnum(
      4,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.setParent = function(value) {
  return jspb.Message.setProto3StringField(this, 1, value);
};


/**
 * optional OverrideInlineSource inline_source = 2;
 * @return {?proto.google.api.serviceusage.v1beta1.OverrideInlineSource}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.getInlineSource = function() {
  return /** @type{?proto.google.api.serviceusage.v1beta1.OverrideInlineSource} */ (
    jspb.Message.getWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.OverrideInlineSource, 2));
};


/**
 * @param {?proto.google.api.serviceusage.v1beta1.OverrideInlineSource|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} returns this
*/
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.setInlineSource = function(value) {
  return jspb.Message.setOneofWrapperField(this, 2, proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.clearInlineSource = function() {
  return this.setInlineSource(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.hasInlineSource = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool force = 3;
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.getForce = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.setForce = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


/**
 * repeated QuotaSafetyCheck force_only = 4;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.getForceOnlyList = function() {
  return /** @type {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} */ (jspb.Message.getRepeatedField(this, 4));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.setForceOnlyList = function(value) {
  return jspb.Message.setField(this, 4, value || []);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaSafetyCheck} value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.addForceOnly = function(value, opt_index) {
  return jspb.Message.addToRepeatedField(this, 4, value, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesRequest.prototype.clearForceOnlyList = function() {
  return this.setForceOnlyList([]);
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    overridesList: jspb.Message.toObjectList(msg.getOverridesList(),
    google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.toObject, includeInstance)
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse;
  return proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.QuotaOverride;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.deserializeBinaryFromReader);
      msg.addOverrides(value);
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getOverridesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.QuotaOverride.serializeBinaryToWriter
    );
  }
};


/**
 * repeated QuotaOverride overrides = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.prototype.getOverridesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.QuotaOverride, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.QuotaOverride>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.prototype.setOverridesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.QuotaOverride=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.QuotaOverride}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.prototype.addOverrides = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.QuotaOverride, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesResponse.prototype.clearOverridesList = function() {
  return this.setOverridesList([]);
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata;
  return proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata}
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportConsumerOverridesMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.repeatedFields_ = [1];



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
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    policiesList: jspb.Message.toObjectList(msg.getPoliciesList(),
    google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy.toObject, includeInstance)
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse;
  return proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy.deserializeBinaryFromReader);
      msg.addPolicies(value);
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
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getPoliciesList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy.serializeBinaryToWriter
    );
  }
};


/**
 * repeated AdminQuotaPolicy policies = 1;
 * @return {!Array<!proto.google.api.serviceusage.v1beta1.AdminQuotaPolicy>}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.prototype.getPoliciesList = function() {
  return /** @type{!Array<!proto.google.api.serviceusage.v1beta1.AdminQuotaPolicy>} */ (
    jspb.Message.getRepeatedWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.AdminQuotaPolicy, 1));
};


/**
 * @param {!Array<!proto.google.api.serviceusage.v1beta1.AdminQuotaPolicy>} value
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.prototype.setPoliciesList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.AdminQuotaPolicy=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.serviceusage.v1beta1.AdminQuotaPolicy}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.prototype.addPolicies = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.serviceusage.v1beta1.AdminQuotaPolicy, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesResponse.prototype.clearPoliciesList = function() {
  return this.setPoliciesList([]);
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
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata;
  return proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata}
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.ImportAdminQuotaPoliciesMetadata.serializeBinaryToWriter = function(message, writer) {
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
proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata;
  return proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata}
 */
proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.CreateAdminQuotaPolicyMetadata.serializeBinaryToWriter = function(message, writer) {
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
proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata;
  return proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata}
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.UpdateAdminQuotaPolicyMetadata.serializeBinaryToWriter = function(message, writer) {
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
proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata}
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata;
  return proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata}
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.DeleteAdminQuotaPolicyMetadata.serializeBinaryToWriter = function(message, writer) {
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
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.toObject = function(includeInstance, msg) {
  var f, obj = {
    parent: jspb.Message.getFieldWithDefault(msg, 1, "")
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
 * @return {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest}
 */
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest;
  return proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest}
 */
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getParent();
  if (f.length > 0) {
    writer.writeString(
      1,
      f
    );
  }
};


/**
 * optional string parent = 1;
 * @return {string}
 */
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.prototype.getParent = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 1, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest} returns this
 */
proto.google.api.serviceusage.v1beta1.GenerateServiceIdentityRequest.prototype.setParent = function(value) {
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
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.toObject = function(includeInstance, msg) {
  var f, obj = {
    identity: (f = msg.getIdentity()) && google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity.toObject(includeInstance, f),
    state: jspb.Message.getFieldWithDefault(msg, 2, 0)
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
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse;
  return proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity;
      reader.readMessage(value,google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity.deserializeBinaryFromReader);
      msg.setIdentity(value);
      break;
    case 2:
      var value = /** @type {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.IdentityState} */ (reader.readEnum());
      msg.setState(value);
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
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getIdentity();
  if (f != null) {
    writer.writeMessage(
      1,
      f,
      google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity.serializeBinaryToWriter
    );
  }
  f = message.getState();
  if (f !== 0.0) {
    writer.writeEnum(
      2,
      f
    );
  }
};


/**
 * @enum {number}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.IdentityState = {
  IDENTITY_STATE_UNSPECIFIED: 0,
  ACTIVE: 1
};

/**
 * optional ServiceIdentity identity = 1;
 * @return {?proto.google.api.serviceusage.v1beta1.ServiceIdentity}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.getIdentity = function() {
  return /** @type{?proto.google.api.serviceusage.v1beta1.ServiceIdentity} */ (
    jspb.Message.getWrapperField(this, google_api_serviceusage_v1beta1_resources_pb.ServiceIdentity, 1));
};


/**
 * @param {?proto.google.api.serviceusage.v1beta1.ServiceIdentity|undefined} value
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse} returns this
*/
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.setIdentity = function(value) {
  return jspb.Message.setWrapperField(this, 1, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.clearIdentity = function() {
  return this.setIdentity(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.hasIdentity = function() {
  return jspb.Message.getField(this, 1) != null;
};


/**
 * optional IdentityState state = 2;
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.IdentityState}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.getState = function() {
  return /** @type {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.IdentityState} */ (jspb.Message.getFieldWithDefault(this, 2, 0));
};


/**
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.IdentityState} value
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse} returns this
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityResponse.prototype.setState = function(value) {
  return jspb.Message.setProto3EnumField(this, 2, value);
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
proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.toObject = function(includeInstance, msg) {
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
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata;
  return proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata}
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.deserializeBinaryFromReader = function(msg, reader) {
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
proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.serviceusage.v1beta1.GetServiceIdentityMetadata.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
};


goog.object.extend(exports, proto.google.api.serviceusage.v1beta1);
