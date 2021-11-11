// source: google/api/expr/v1beta1/decl.proto
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

var google_api_expr_v1beta1_expr_pb = require('../../../../google/api/expr/v1beta1/expr_pb.js');
goog.object.extend(proto, google_api_expr_v1beta1_expr_pb);
goog.exportSymbol('proto.google.api.expr.v1beta1.Decl', null, global);
goog.exportSymbol('proto.google.api.expr.v1beta1.Decl.KindCase', null, global);
goog.exportSymbol('proto.google.api.expr.v1beta1.DeclType', null, global);
goog.exportSymbol('proto.google.api.expr.v1beta1.FunctionDecl', null, global);
goog.exportSymbol('proto.google.api.expr.v1beta1.IdentDecl', null, global);
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
proto.google.api.expr.v1beta1.Decl = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, proto.google.api.expr.v1beta1.Decl.oneofGroups_);
};
goog.inherits(proto.google.api.expr.v1beta1.Decl, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.expr.v1beta1.Decl.displayName = 'proto.google.api.expr.v1beta1.Decl';
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
proto.google.api.expr.v1beta1.DeclType = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.expr.v1beta1.DeclType.repeatedFields_, null);
};
goog.inherits(proto.google.api.expr.v1beta1.DeclType, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.expr.v1beta1.DeclType.displayName = 'proto.google.api.expr.v1beta1.DeclType';
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
proto.google.api.expr.v1beta1.IdentDecl = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, null, null);
};
goog.inherits(proto.google.api.expr.v1beta1.IdentDecl, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.expr.v1beta1.IdentDecl.displayName = 'proto.google.api.expr.v1beta1.IdentDecl';
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
proto.google.api.expr.v1beta1.FunctionDecl = function(opt_data) {
  jspb.Message.initialize(this, opt_data, 0, -1, proto.google.api.expr.v1beta1.FunctionDecl.repeatedFields_, null);
};
goog.inherits(proto.google.api.expr.v1beta1.FunctionDecl, jspb.Message);
if (goog.DEBUG && !COMPILED) {
  /**
   * @public
   * @override
   */
  proto.google.api.expr.v1beta1.FunctionDecl.displayName = 'proto.google.api.expr.v1beta1.FunctionDecl';
}

/**
 * Oneof group definitions for this message. Each group defines the field
 * numbers belonging to that group. When of these fields' value is set, all
 * other fields in the group are cleared. During deserialization, if multiple
 * fields are encountered for a group, only the last value seen will be kept.
 * @private {!Array<!Array<number>>}
 * @const
 */
proto.google.api.expr.v1beta1.Decl.oneofGroups_ = [[4,5]];

/**
 * @enum {number}
 */
proto.google.api.expr.v1beta1.Decl.KindCase = {
  KIND_NOT_SET: 0,
  IDENT: 4,
  FUNCTION: 5
};

/**
 * @return {proto.google.api.expr.v1beta1.Decl.KindCase}
 */
proto.google.api.expr.v1beta1.Decl.prototype.getKindCase = function() {
  return /** @type {proto.google.api.expr.v1beta1.Decl.KindCase} */(jspb.Message.computeOneofCase(this, proto.google.api.expr.v1beta1.Decl.oneofGroups_[0]));
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
proto.google.api.expr.v1beta1.Decl.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.expr.v1beta1.Decl.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.expr.v1beta1.Decl} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.Decl.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, 0),
    name: jspb.Message.getFieldWithDefault(msg, 2, ""),
    doc: jspb.Message.getFieldWithDefault(msg, 3, ""),
    ident: (f = msg.getIdent()) && proto.google.api.expr.v1beta1.IdentDecl.toObject(includeInstance, f),
    pb_function: (f = msg.getFunction()) && proto.google.api.expr.v1beta1.FunctionDecl.toObject(includeInstance, f)
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
 * @return {!proto.google.api.expr.v1beta1.Decl}
 */
proto.google.api.expr.v1beta1.Decl.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.expr.v1beta1.Decl;
  return proto.google.api.expr.v1beta1.Decl.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.expr.v1beta1.Decl} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.expr.v1beta1.Decl}
 */
proto.google.api.expr.v1beta1.Decl.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setName(value);
      break;
    case 3:
      var value = /** @type {string} */ (reader.readString());
      msg.setDoc(value);
      break;
    case 4:
      var value = new proto.google.api.expr.v1beta1.IdentDecl;
      reader.readMessage(value,proto.google.api.expr.v1beta1.IdentDecl.deserializeBinaryFromReader);
      msg.setIdent(value);
      break;
    case 5:
      var value = new proto.google.api.expr.v1beta1.FunctionDecl;
      reader.readMessage(value,proto.google.api.expr.v1beta1.FunctionDecl.deserializeBinaryFromReader);
      msg.setFunction(value);
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
proto.google.api.expr.v1beta1.Decl.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.expr.v1beta1.Decl.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.expr.v1beta1.Decl} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.Decl.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f !== 0) {
    writer.writeInt32(
      1,
      f
    );
  }
  f = message.getName();
  if (f.length > 0) {
    writer.writeString(
      2,
      f
    );
  }
  f = message.getDoc();
  if (f.length > 0) {
    writer.writeString(
      3,
      f
    );
  }
  f = message.getIdent();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      proto.google.api.expr.v1beta1.IdentDecl.serializeBinaryToWriter
    );
  }
  f = message.getFunction();
  if (f != null) {
    writer.writeMessage(
      5,
      f,
      proto.google.api.expr.v1beta1.FunctionDecl.serializeBinaryToWriter
    );
  }
};


/**
 * optional int32 id = 1;
 * @return {number}
 */
proto.google.api.expr.v1beta1.Decl.prototype.getId = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.google.api.expr.v1beta1.Decl} returns this
 */
proto.google.api.expr.v1beta1.Decl.prototype.setId = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional string name = 2;
 * @return {string}
 */
proto.google.api.expr.v1beta1.Decl.prototype.getName = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.expr.v1beta1.Decl} returns this
 */
proto.google.api.expr.v1beta1.Decl.prototype.setName = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * optional string doc = 3;
 * @return {string}
 */
proto.google.api.expr.v1beta1.Decl.prototype.getDoc = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 3, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.expr.v1beta1.Decl} returns this
 */
proto.google.api.expr.v1beta1.Decl.prototype.setDoc = function(value) {
  return jspb.Message.setProto3StringField(this, 3, value);
};


/**
 * optional IdentDecl ident = 4;
 * @return {?proto.google.api.expr.v1beta1.IdentDecl}
 */
proto.google.api.expr.v1beta1.Decl.prototype.getIdent = function() {
  return /** @type{?proto.google.api.expr.v1beta1.IdentDecl} */ (
    jspb.Message.getWrapperField(this, proto.google.api.expr.v1beta1.IdentDecl, 4));
};


/**
 * @param {?proto.google.api.expr.v1beta1.IdentDecl|undefined} value
 * @return {!proto.google.api.expr.v1beta1.Decl} returns this
*/
proto.google.api.expr.v1beta1.Decl.prototype.setIdent = function(value) {
  return jspb.Message.setOneofWrapperField(this, 4, proto.google.api.expr.v1beta1.Decl.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.expr.v1beta1.Decl} returns this
 */
proto.google.api.expr.v1beta1.Decl.prototype.clearIdent = function() {
  return this.setIdent(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.expr.v1beta1.Decl.prototype.hasIdent = function() {
  return jspb.Message.getField(this, 4) != null;
};


/**
 * optional FunctionDecl function = 5;
 * @return {?proto.google.api.expr.v1beta1.FunctionDecl}
 */
proto.google.api.expr.v1beta1.Decl.prototype.getFunction = function() {
  return /** @type{?proto.google.api.expr.v1beta1.FunctionDecl} */ (
    jspb.Message.getWrapperField(this, proto.google.api.expr.v1beta1.FunctionDecl, 5));
};


/**
 * @param {?proto.google.api.expr.v1beta1.FunctionDecl|undefined} value
 * @return {!proto.google.api.expr.v1beta1.Decl} returns this
*/
proto.google.api.expr.v1beta1.Decl.prototype.setFunction = function(value) {
  return jspb.Message.setOneofWrapperField(this, 5, proto.google.api.expr.v1beta1.Decl.oneofGroups_[0], value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.expr.v1beta1.Decl} returns this
 */
proto.google.api.expr.v1beta1.Decl.prototype.clearFunction = function() {
  return this.setFunction(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.expr.v1beta1.Decl.prototype.hasFunction = function() {
  return jspb.Message.getField(this, 5) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.expr.v1beta1.DeclType.repeatedFields_ = [4];



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
proto.google.api.expr.v1beta1.DeclType.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.expr.v1beta1.DeclType.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.expr.v1beta1.DeclType} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.DeclType.toObject = function(includeInstance, msg) {
  var f, obj = {
    id: jspb.Message.getFieldWithDefault(msg, 1, 0),
    type: jspb.Message.getFieldWithDefault(msg, 2, ""),
    typeParamsList: jspb.Message.toObjectList(msg.getTypeParamsList(),
    proto.google.api.expr.v1beta1.DeclType.toObject, includeInstance)
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
 * @return {!proto.google.api.expr.v1beta1.DeclType}
 */
proto.google.api.expr.v1beta1.DeclType.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.expr.v1beta1.DeclType;
  return proto.google.api.expr.v1beta1.DeclType.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.expr.v1beta1.DeclType} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.expr.v1beta1.DeclType}
 */
proto.google.api.expr.v1beta1.DeclType.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = /** @type {number} */ (reader.readInt32());
      msg.setId(value);
      break;
    case 2:
      var value = /** @type {string} */ (reader.readString());
      msg.setType(value);
      break;
    case 4:
      var value = new proto.google.api.expr.v1beta1.DeclType;
      reader.readMessage(value,proto.google.api.expr.v1beta1.DeclType.deserializeBinaryFromReader);
      msg.addTypeParams(value);
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
proto.google.api.expr.v1beta1.DeclType.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.expr.v1beta1.DeclType.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.expr.v1beta1.DeclType} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.DeclType.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getId();
  if (f !== 0) {
    writer.writeInt32(
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
  f = message.getTypeParamsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      4,
      f,
      proto.google.api.expr.v1beta1.DeclType.serializeBinaryToWriter
    );
  }
};


/**
 * optional int32 id = 1;
 * @return {number}
 */
proto.google.api.expr.v1beta1.DeclType.prototype.getId = function() {
  return /** @type {number} */ (jspb.Message.getFieldWithDefault(this, 1, 0));
};


/**
 * @param {number} value
 * @return {!proto.google.api.expr.v1beta1.DeclType} returns this
 */
proto.google.api.expr.v1beta1.DeclType.prototype.setId = function(value) {
  return jspb.Message.setProto3IntField(this, 1, value);
};


/**
 * optional string type = 2;
 * @return {string}
 */
proto.google.api.expr.v1beta1.DeclType.prototype.getType = function() {
  return /** @type {string} */ (jspb.Message.getFieldWithDefault(this, 2, ""));
};


/**
 * @param {string} value
 * @return {!proto.google.api.expr.v1beta1.DeclType} returns this
 */
proto.google.api.expr.v1beta1.DeclType.prototype.setType = function(value) {
  return jspb.Message.setProto3StringField(this, 2, value);
};


/**
 * repeated DeclType type_params = 4;
 * @return {!Array<!proto.google.api.expr.v1beta1.DeclType>}
 */
proto.google.api.expr.v1beta1.DeclType.prototype.getTypeParamsList = function() {
  return /** @type{!Array<!proto.google.api.expr.v1beta1.DeclType>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.google.api.expr.v1beta1.DeclType, 4));
};


/**
 * @param {!Array<!proto.google.api.expr.v1beta1.DeclType>} value
 * @return {!proto.google.api.expr.v1beta1.DeclType} returns this
*/
proto.google.api.expr.v1beta1.DeclType.prototype.setTypeParamsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 4, value);
};


/**
 * @param {!proto.google.api.expr.v1beta1.DeclType=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.expr.v1beta1.DeclType}
 */
proto.google.api.expr.v1beta1.DeclType.prototype.addTypeParams = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 4, opt_value, proto.google.api.expr.v1beta1.DeclType, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.expr.v1beta1.DeclType} returns this
 */
proto.google.api.expr.v1beta1.DeclType.prototype.clearTypeParamsList = function() {
  return this.setTypeParamsList([]);
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
proto.google.api.expr.v1beta1.IdentDecl.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.expr.v1beta1.IdentDecl.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.expr.v1beta1.IdentDecl} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.IdentDecl.toObject = function(includeInstance, msg) {
  var f, obj = {
    type: (f = msg.getType()) && proto.google.api.expr.v1beta1.DeclType.toObject(includeInstance, f),
    value: (f = msg.getValue()) && google_api_expr_v1beta1_expr_pb.Expr.toObject(includeInstance, f)
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
 * @return {!proto.google.api.expr.v1beta1.IdentDecl}
 */
proto.google.api.expr.v1beta1.IdentDecl.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.expr.v1beta1.IdentDecl;
  return proto.google.api.expr.v1beta1.IdentDecl.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.expr.v1beta1.IdentDecl} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.expr.v1beta1.IdentDecl}
 */
proto.google.api.expr.v1beta1.IdentDecl.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 3:
      var value = new proto.google.api.expr.v1beta1.DeclType;
      reader.readMessage(value,proto.google.api.expr.v1beta1.DeclType.deserializeBinaryFromReader);
      msg.setType(value);
      break;
    case 4:
      var value = new google_api_expr_v1beta1_expr_pb.Expr;
      reader.readMessage(value,google_api_expr_v1beta1_expr_pb.Expr.deserializeBinaryFromReader);
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
proto.google.api.expr.v1beta1.IdentDecl.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.expr.v1beta1.IdentDecl.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.expr.v1beta1.IdentDecl} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.IdentDecl.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getType();
  if (f != null) {
    writer.writeMessage(
      3,
      f,
      proto.google.api.expr.v1beta1.DeclType.serializeBinaryToWriter
    );
  }
  f = message.getValue();
  if (f != null) {
    writer.writeMessage(
      4,
      f,
      google_api_expr_v1beta1_expr_pb.Expr.serializeBinaryToWriter
    );
  }
};


/**
 * optional DeclType type = 3;
 * @return {?proto.google.api.expr.v1beta1.DeclType}
 */
proto.google.api.expr.v1beta1.IdentDecl.prototype.getType = function() {
  return /** @type{?proto.google.api.expr.v1beta1.DeclType} */ (
    jspb.Message.getWrapperField(this, proto.google.api.expr.v1beta1.DeclType, 3));
};


/**
 * @param {?proto.google.api.expr.v1beta1.DeclType|undefined} value
 * @return {!proto.google.api.expr.v1beta1.IdentDecl} returns this
*/
proto.google.api.expr.v1beta1.IdentDecl.prototype.setType = function(value) {
  return jspb.Message.setWrapperField(this, 3, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.expr.v1beta1.IdentDecl} returns this
 */
proto.google.api.expr.v1beta1.IdentDecl.prototype.clearType = function() {
  return this.setType(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.expr.v1beta1.IdentDecl.prototype.hasType = function() {
  return jspb.Message.getField(this, 3) != null;
};


/**
 * optional Expr value = 4;
 * @return {?proto.google.api.expr.v1beta1.Expr}
 */
proto.google.api.expr.v1beta1.IdentDecl.prototype.getValue = function() {
  return /** @type{?proto.google.api.expr.v1beta1.Expr} */ (
    jspb.Message.getWrapperField(this, google_api_expr_v1beta1_expr_pb.Expr, 4));
};


/**
 * @param {?proto.google.api.expr.v1beta1.Expr|undefined} value
 * @return {!proto.google.api.expr.v1beta1.IdentDecl} returns this
*/
proto.google.api.expr.v1beta1.IdentDecl.prototype.setValue = function(value) {
  return jspb.Message.setWrapperField(this, 4, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.expr.v1beta1.IdentDecl} returns this
 */
proto.google.api.expr.v1beta1.IdentDecl.prototype.clearValue = function() {
  return this.setValue(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.expr.v1beta1.IdentDecl.prototype.hasValue = function() {
  return jspb.Message.getField(this, 4) != null;
};



/**
 * List of repeated fields within this message type.
 * @private {!Array<number>}
 * @const
 */
proto.google.api.expr.v1beta1.FunctionDecl.repeatedFields_ = [1];



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
proto.google.api.expr.v1beta1.FunctionDecl.prototype.toObject = function(opt_includeInstance) {
  return proto.google.api.expr.v1beta1.FunctionDecl.toObject(opt_includeInstance, this);
};


/**
 * Static version of the {@see toObject} method.
 * @param {boolean|undefined} includeInstance Deprecated. Whether to include
 *     the JSPB instance for transitional soy proto support:
 *     http://goto/soy-param-migration
 * @param {!proto.google.api.expr.v1beta1.FunctionDecl} msg The msg instance to transform.
 * @return {!Object}
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.FunctionDecl.toObject = function(includeInstance, msg) {
  var f, obj = {
    argsList: jspb.Message.toObjectList(msg.getArgsList(),
    proto.google.api.expr.v1beta1.IdentDecl.toObject, includeInstance),
    returnType: (f = msg.getReturnType()) && proto.google.api.expr.v1beta1.DeclType.toObject(includeInstance, f),
    receiverFunction: jspb.Message.getBooleanFieldWithDefault(msg, 3, false)
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
 * @return {!proto.google.api.expr.v1beta1.FunctionDecl}
 */
proto.google.api.expr.v1beta1.FunctionDecl.deserializeBinary = function(bytes) {
  var reader = new jspb.BinaryReader(bytes);
  var msg = new proto.google.api.expr.v1beta1.FunctionDecl;
  return proto.google.api.expr.v1beta1.FunctionDecl.deserializeBinaryFromReader(msg, reader);
};


/**
 * Deserializes binary data (in protobuf wire format) from the
 * given reader into the given message object.
 * @param {!proto.google.api.expr.v1beta1.FunctionDecl} msg The message object to deserialize into.
 * @param {!jspb.BinaryReader} reader The BinaryReader to use.
 * @return {!proto.google.api.expr.v1beta1.FunctionDecl}
 */
proto.google.api.expr.v1beta1.FunctionDecl.deserializeBinaryFromReader = function(msg, reader) {
  while (reader.nextField()) {
    if (reader.isEndGroup()) {
      break;
    }
    var field = reader.getFieldNumber();
    switch (field) {
    case 1:
      var value = new proto.google.api.expr.v1beta1.IdentDecl;
      reader.readMessage(value,proto.google.api.expr.v1beta1.IdentDecl.deserializeBinaryFromReader);
      msg.addArgs(value);
      break;
    case 2:
      var value = new proto.google.api.expr.v1beta1.DeclType;
      reader.readMessage(value,proto.google.api.expr.v1beta1.DeclType.deserializeBinaryFromReader);
      msg.setReturnType(value);
      break;
    case 3:
      var value = /** @type {boolean} */ (reader.readBool());
      msg.setReceiverFunction(value);
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
proto.google.api.expr.v1beta1.FunctionDecl.prototype.serializeBinary = function() {
  var writer = new jspb.BinaryWriter();
  proto.google.api.expr.v1beta1.FunctionDecl.serializeBinaryToWriter(this, writer);
  return writer.getResultBuffer();
};


/**
 * Serializes the given message to binary data (in protobuf wire
 * format), writing to the given BinaryWriter.
 * @param {!proto.google.api.expr.v1beta1.FunctionDecl} message
 * @param {!jspb.BinaryWriter} writer
 * @suppress {unusedLocalVariables} f is only used for nested messages
 */
proto.google.api.expr.v1beta1.FunctionDecl.serializeBinaryToWriter = function(message, writer) {
  var f = undefined;
  f = message.getArgsList();
  if (f.length > 0) {
    writer.writeRepeatedMessage(
      1,
      f,
      proto.google.api.expr.v1beta1.IdentDecl.serializeBinaryToWriter
    );
  }
  f = message.getReturnType();
  if (f != null) {
    writer.writeMessage(
      2,
      f,
      proto.google.api.expr.v1beta1.DeclType.serializeBinaryToWriter
    );
  }
  f = message.getReceiverFunction();
  if (f) {
    writer.writeBool(
      3,
      f
    );
  }
};


/**
 * repeated IdentDecl args = 1;
 * @return {!Array<!proto.google.api.expr.v1beta1.IdentDecl>}
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.getArgsList = function() {
  return /** @type{!Array<!proto.google.api.expr.v1beta1.IdentDecl>} */ (
    jspb.Message.getRepeatedWrapperField(this, proto.google.api.expr.v1beta1.IdentDecl, 1));
};


/**
 * @param {!Array<!proto.google.api.expr.v1beta1.IdentDecl>} value
 * @return {!proto.google.api.expr.v1beta1.FunctionDecl} returns this
*/
proto.google.api.expr.v1beta1.FunctionDecl.prototype.setArgsList = function(value) {
  return jspb.Message.setRepeatedWrapperField(this, 1, value);
};


/**
 * @param {!proto.google.api.expr.v1beta1.IdentDecl=} opt_value
 * @param {number=} opt_index
 * @return {!proto.google.api.expr.v1beta1.IdentDecl}
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.addArgs = function(opt_value, opt_index) {
  return jspb.Message.addToRepeatedWrapperField(this, 1, opt_value, proto.google.api.expr.v1beta1.IdentDecl, opt_index);
};


/**
 * Clears the list making it empty but non-null.
 * @return {!proto.google.api.expr.v1beta1.FunctionDecl} returns this
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.clearArgsList = function() {
  return this.setArgsList([]);
};


/**
 * optional DeclType return_type = 2;
 * @return {?proto.google.api.expr.v1beta1.DeclType}
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.getReturnType = function() {
  return /** @type{?proto.google.api.expr.v1beta1.DeclType} */ (
    jspb.Message.getWrapperField(this, proto.google.api.expr.v1beta1.DeclType, 2));
};


/**
 * @param {?proto.google.api.expr.v1beta1.DeclType|undefined} value
 * @return {!proto.google.api.expr.v1beta1.FunctionDecl} returns this
*/
proto.google.api.expr.v1beta1.FunctionDecl.prototype.setReturnType = function(value) {
  return jspb.Message.setWrapperField(this, 2, value);
};


/**
 * Clears the message field making it undefined.
 * @return {!proto.google.api.expr.v1beta1.FunctionDecl} returns this
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.clearReturnType = function() {
  return this.setReturnType(undefined);
};


/**
 * Returns whether this field is set.
 * @return {boolean}
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.hasReturnType = function() {
  return jspb.Message.getField(this, 2) != null;
};


/**
 * optional bool receiver_function = 3;
 * @return {boolean}
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.getReceiverFunction = function() {
  return /** @type {boolean} */ (jspb.Message.getBooleanFieldWithDefault(this, 3, false));
};


/**
 * @param {boolean} value
 * @return {!proto.google.api.expr.v1beta1.FunctionDecl} returns this
 */
proto.google.api.expr.v1beta1.FunctionDecl.prototype.setReceiverFunction = function(value) {
  return jspb.Message.setProto3BooleanField(this, 3, value);
};


goog.object.extend(exports, proto.google.api.expr.v1beta1);
