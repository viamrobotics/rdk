// Copyright 2023 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package protoyaml

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/bufbuild/protovalidate-go"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"gopkg.in/yaml.v3"
)

const atTypeFieldName = "@type"

// Validator is an interface for validating a Protobuf message produced from a given YAML node.
type Validator interface {
	// Validate the given message.
	Validate(message proto.Message) error
}

// UnmarshalOptions is a configurable YAML format parser for Protobuf messages.
type UnmarshalOptions struct {
	// The path for the data being unmarshaled.
	//
	// If set, this will be used when producing error messages.
	Path string
	// Validator is a validator to run after unmarshaling a message.
	Validator Validator
	// Resolver is the Protobuf type resolver to use.
	Resolver interface {
		protoregistry.MessageTypeResolver
		protoregistry.ExtensionTypeResolver
	}
}

// Unmarshal a Protobuf message from the given YAML data.
func Unmarshal(data []byte, message proto.Message) error {
	return (UnmarshalOptions{}).Unmarshal(data, message)
}

// Unmarshal a Protobuf message from the given YAML data.
func (o UnmarshalOptions) Unmarshal(data []byte, message proto.Message) error {
	var yamlFile yaml.Node
	if err := yaml.Unmarshal(data, &yamlFile); err != nil {
		return err
	}
	return o.unmarshalNode(&yamlFile, message, data)
}

func (o UnmarshalOptions) unmarshalNode(node *yaml.Node, message proto.Message, data []byte) error {
	if node.Kind == 0 {
		return nil
	}
	unm := &unmarshaler{
		options:   o,
		custom:    make(map[protoreflect.FullName]customUnmarshaler),
		validator: o.Validator,
		lines:     strings.Split(string(data), "\n"),
	}

	addWktUnmarshalers(unm.custom)

	// Unwrap the document node
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) != 1 {
			return errors.New("expected exactly one node in document")
		}
		node = node.Content[0]
	}

	unm.unmarshalMessage(node, message, false)
	if unm.validator != nil {
		err := unm.validator.Validate(message)
		var verr *protovalidate.ValidationError
		switch {
		case err == nil: // Valid.
		case errors.As(err, &verr):
			for _, violation := range verr.Violations {
				closest := nodeClosestToPath(node, message.ProtoReflect().Descriptor(), violation.GetFieldPath(), violation.GetForKey())
				unm.addError(closest, &violationError{
					Violation: violation,
				})
			}
		default:
			unm.addError(node, err)
		}
	}

	if len(unm.errors) > 0 {
		return unmarshalErrors(unm.errors)
	}
	return nil
}

type unmarshaler struct {
	options   UnmarshalOptions
	errors    []error
	custom    map[protoreflect.FullName]customUnmarshaler
	validator Validator
	lines     []string
}

func (u *unmarshaler) addError(node *yaml.Node, err error) {
	u.errors = append(u.errors, &nodeError{
		Path:  u.options.Path,
		Node:  node,
		cause: err,
		line:  u.lines[node.Line-1],
	})
}
func (u *unmarshaler) addErrorf(node *yaml.Node, format string, args ...interface{}) {
	u.addError(node, fmt.Errorf(format, args...))
}

func (u *unmarshaler) checkKind(node *yaml.Node, expected yaml.Kind) bool {
	if node.Kind != expected {
		u.addErrorf(node, "expected %v, got %v", getNodeKind(expected), getNodeKind(node.Kind))
		return false
	}
	return true
}

func (u *unmarshaler) checkTag(node *yaml.Node, expected string) {
	if node.Tag != "" && node.Tag != expected {
		u.addErrorf(node, "expected tag %v, got %v", expected, node.Tag)
	}
}

func (u *unmarshaler) findAnyTypeURL(node *yaml.Node) string {
	typeURL := ""
	for i := 1; i < len(node.Content); i += 2 {
		keyNode := node.Content[i-1]
		valueNode := node.Content[i]
		if keyNode.Value == atTypeFieldName && u.checkKind(valueNode, yaml.ScalarNode) {
			typeURL = valueNode.Value
			break
		}
	}
	return typeURL
}

func (u *unmarshaler) resolveAnyType(typeURL string) (protoreflect.MessageType, error) {
	// Get the message type.
	var msgType protoreflect.MessageType
	var err error
	if u.options.Resolver != nil {
		msgType, err = u.options.Resolver.FindMessageByURL(typeURL)
	} else { // Use the global registry.
		msgType, err = protoregistry.GlobalTypes.FindMessageByURL(typeURL)
	}
	if err != nil {
		return nil, err
	}
	return msgType, nil
}

func (u *unmarshaler) findAnyType(node *yaml.Node) (protoreflect.MessageType, error) {
	typeURL := u.findAnyTypeURL(node)
	if typeURL == "" {
		return nil, errors.New("missing @type field")
	}
	return u.resolveAnyType(typeURL)
}

// Unmarshal the field based on the field kind, ignoring IsList and IsMap,
// which are handled by the caller.
func (u *unmarshaler) unmarshalScalar(
	node *yaml.Node,
	field protoreflect.FieldDescriptor,
	forKey bool,
) protoreflect.Value {
	switch field.Kind() {
	case protoreflect.BoolKind:
		return protoreflect.ValueOfBool(u.unmarshalBool(node, forKey))
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		return protoreflect.ValueOfInt32(int32(u.unmarshalInteger(node, 32)))
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return protoreflect.ValueOfInt64(u.unmarshalInteger(node, 64))
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		return protoreflect.ValueOfUint32(uint32(u.unmarshalUnsigned(node, 32)))
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return protoreflect.ValueOfUint64(u.unmarshalUnsigned(node, 64))
	case protoreflect.FloatKind:
		return protoreflect.ValueOfFloat32(float32(u.unmarshalFloat(node, 32)))
	case protoreflect.DoubleKind:
		return protoreflect.ValueOfFloat64(u.unmarshalFloat(node, 64))
	case protoreflect.StringKind:
		u.checkKind(node, yaml.ScalarNode)
		return protoreflect.ValueOfString(node.Value)
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes(u.unmarshalBytes(node))
	case protoreflect.EnumKind:
		return protoreflect.ValueOfEnum(u.unmarshalEnum(node, field))
	default:
		u.addErrorf(node, "unimplemented scalar type %v", field.Kind())
		return protoreflect.Value{}
	}
}

// Base64 decodes the given node value.
func (u *unmarshaler) unmarshalBytes(node *yaml.Node) []byte {
	if !u.checkKind(node, yaml.ScalarNode) {
		return nil
	}

	enc := base64.StdEncoding
	if strings.ContainsAny(node.Value, "-_") {
		enc = base64.URLEncoding
	}
	if len(node.Value)%4 != 0 {
		enc = enc.WithPadding(base64.NoPadding)
	}

	// base64 decode the value.
	data, err := enc.DecodeString(node.Value)
	if err != nil {
		u.addErrorf(node, "invalid base64: %v", err)
	}
	return data
}

// Unmarshal raw `true` or `false` values, only allowing for strings for keys.
func (u *unmarshaler) unmarshalBool(node *yaml.Node, forKey bool) bool {
	if u.checkKind(node, yaml.ScalarNode) {
		switch node.Value {
		case "true":
			if !forKey {
				u.checkTag(node, "!!bool")
			}
			return true
		case "false":
			if !forKey {
				u.checkTag(node, "!!bool")
			}
			return false
		default:
			u.addErrorf(node, "expected bool, got %#v", node.Value)
		}
	}
	return false
}

// Unmarshal the given node into an enum value.
//
// Accepts either the enum name or number.
func (u *unmarshaler) unmarshalEnum(node *yaml.Node, field protoreflect.FieldDescriptor) protoreflect.EnumNumber {
	u.checkKind(node, yaml.ScalarNode)
	// Get the enum descriptor.
	enumDesc := field.Enum()
	if enumDesc.FullName() == "google.protobuf.NullValue" {
		return 0
	}
	// Get the enum value.
	enumVal := enumDesc.Values().ByName(protoreflect.Name(node.Value))
	if enumVal == nil {
		lit, err := parseIntLiteral(node.Value)
		if err != nil {
			u.addErrorf(node, "unknown enum value %#v, expected one of %v", node.Value,
				getEnumValueNames(enumDesc.Values()))
		} else if err := lit.checkI32(field); err != nil {
			u.addErrorf(node, "%w, expected one of %v", err,
				getEnumValueNames(enumDesc.Values()))
		}
		num := protoreflect.EnumNumber(lit.value)
		if lit.negative {
			num = -num
		}
		return num
	}
	return enumVal.Number()
}

// Unmarshal the given node into a float with the given bits.
func (u *unmarshaler) unmarshalFloat(node *yaml.Node, bits int) float64 {
	if !u.checkKind(node, yaml.ScalarNode) {
		return 0
	}

	parsed, err := strconv.ParseFloat(node.Value, bits)
	if err != nil {
		u.addErrorf(node, "invalid float: %v", err)
	}
	return parsed
}

// Unmarshal the given node into an unsigned integer with the given bits.
func (u *unmarshaler) unmarshalUnsigned(node *yaml.Node, bits int) uint64 {
	if !u.checkKind(node, yaml.ScalarNode) {
		return 0
	}

	parsed, err := parseUintLiteral(node.Value)
	if err != nil {
		u.addErrorf(node, "invalid integer: %v", err)
	}
	if bits < 64 && parsed >= 1<<bits {
		u.addErrorf(node, "integer is too large: > %v", 1<<bits-1)
	}
	return parsed
}

// Unmarshal the given node into a signed integer with the given bits.
func (u *unmarshaler) unmarshalInteger(node *yaml.Node, bits int) int64 {
	if !u.checkKind(node, yaml.ScalarNode) {
		return 0
	}

	lit, err := parseIntLiteral(node.Value)
	if err != nil {
		u.addErrorf(node, "invalid integer: %v", err)
	}
	if lit.negative {
		if lit.value <= 1<<(bits-1) {
			return -int64(lit.value)
		}
		u.addErrorf(node, "integer is too small: < %v", -(1 << (bits - 1)))
	} else if lit.value >= 1<<(bits-1) {
		u.addErrorf(node, "integer is too large: > %v", 1<<(bits-1)-1)
	}
	return int64(lit.value)
}

func getFieldNames(fields protoreflect.FieldDescriptors) []protoreflect.Name {
	names := make([]protoreflect.Name, 0, fields.Len())
	for i := 0; i < fields.Len(); i++ {
		names = append(names, fields.Get(i).Name())
		if i > 5 {
			names = append(names, protoreflect.Name("..."))
			break
		}
	}
	return names
}

func getEnumValueNames(values protoreflect.EnumValueDescriptors) []protoreflect.Name {
	names := make([]protoreflect.Name, 0, values.Len())
	for i := 0; i < values.Len(); i++ {
		names = append(names, values.Get(i).Name())
		if i > 5 {
			names = append(names, protoreflect.Name("..."))
			break
		}
	}
	return names
}

func getNodeKind(kind yaml.Kind) string {
	switch kind {
	case yaml.DocumentNode:
		return "document"
	case yaml.SequenceNode:
		return "sequence"
	case yaml.MappingNode:
		return "mapping"
	case yaml.ScalarNode:
		return "scalar"
	case yaml.AliasNode:
		return "alias"
	}
	return fmt.Sprintf("unknown(%d)", kind)
}

// Parses Octal, Hex, Binary, Decimal, and Unsigned Integer Float literals.
//
// Conversion through JSON/YAML may have converted integers into floats, including
// exponential notation. This function will parse those values back into integers
// if possible.
func parseUintLiteral(value string) (uint64, error) {
	base := 10
	if len(value) >= 2 && strings.HasPrefix(value, "0") {
		switch value[1] {
		case 'x', 'X':
			base = 16
			value = value[2:]
		case 'o', 'O':
			base = 8
			value = value[2:]
		case 'b', 'B':
			base = 2
			value = value[2:]
		}
	}

	parsed, err := strconv.ParseUint(value, base, 64)
	if err != nil {
		parsedFloat, floatErr := strconv.ParseFloat(value, 64)
		if floatErr != nil || parsedFloat < 0 || math.IsInf(parsedFloat, 0) || math.IsNaN(parsedFloat) {
			return 0, err
		}
		// See if it's actually an integer.
		parsed = uint64(parsedFloat)
		if float64(parsed) != parsedFloat || parsed >= (1<<53) {
			return parsed, errors.New("precision loss")
		}
	}
	return parsed, nil
}

type intLit struct {
	negative bool
	value    uint64
}

func (lit intLit) checkI32(field protoreflect.FieldDescriptor) error {
	switch {
	case lit.negative && lit.value > 1<<31: // Underflow.
		return fmt.Errorf("expected int32 for %v, got int64", field.FullName())
	case !lit.negative && lit.value >= 1<<31: // Overflow.
		return fmt.Errorf("expected int32 for %v, got int64", field.FullName())
	}
	return nil
}

func parseIntLiteral(value string) (intLit, error) {
	var lit intLit
	if strings.HasPrefix(value, "-") {
		lit.negative = true
		value = value[1:]
	}
	var err error
	lit.value, err = parseUintLiteral(value)
	return lit, err
}

// Searches for the field with the given 'key' first by Name, then by JSONName,
// and finally by Number.
func findField(key string, fields protoreflect.FieldDescriptors) protoreflect.FieldDescriptor {
	if field := fields.ByName(protoreflect.Name(key)); field != nil {
		return field
	}
	if field := fields.ByJSONName(key); field != nil {
		return field
	}
	num, err := strconv.ParseInt(key, 10, 32)
	if err == nil {
		if field := fields.ByNumber(protoreflect.FieldNumber(num)); field != nil {
			return field
		}
	}
	return nil
}

// Unmarshal a field, handling isList/isMap.
func (u *unmarshaler) unmarshalField(node *yaml.Node, field protoreflect.FieldDescriptor, message proto.Message) {
	switch {
	case field.IsList():
		u.unmarshalList(node, field, message.ProtoReflect().Mutable(field).List())
	case field.IsMap():
		u.unmarshalMap(node, field, message.ProtoReflect().Mutable(field).Map())
	case field.Kind() == protoreflect.MessageKind:
		u.unmarshalMessage(node, message.ProtoReflect().Mutable(field).Message().Interface(), false)
	default:
		message.ProtoReflect().Set(field, u.unmarshalScalar(node, field, false))
	}
}

// Unmarshal the list, with explicit handling for lists of messages.
func (u *unmarshaler) unmarshalList(node *yaml.Node, field protoreflect.FieldDescriptor, list protoreflect.List) {
	if u.checkKind(node, yaml.SequenceNode) {
		switch field.Kind() {
		case protoreflect.MessageKind, protoreflect.GroupKind:
			for _, itemNode := range node.Content {
				msgVal := list.NewElement()
				u.unmarshalMessage(itemNode, msgVal.Message().Interface(), false)
				list.Append(msgVal)
			}
		default:
			for _, itemNode := range node.Content {
				list.Append(u.unmarshalScalar(itemNode, field, false))
			}
		}
	}
}

// Unmarshal the map, with explicit handling for maps to messages.
func (u *unmarshaler) unmarshalMap(node *yaml.Node, field protoreflect.FieldDescriptor, mapVal protoreflect.Map) {
	if u.checkKind(node, yaml.MappingNode) {
		mapKeyField := field.MapKey()
		mapValueField := field.MapValue()
		for i := 1; i < len(node.Content); i += 2 {
			keyNode := node.Content[i-1]
			valueNode := node.Content[i]
			mapKey := u.unmarshalScalar(keyNode, mapKeyField, true)
			switch mapValueField.Kind() {
			case protoreflect.MessageKind, protoreflect.GroupKind:
				mapValue := mapVal.NewValue()
				u.unmarshalMessage(valueNode, mapValue.Message().Interface(), false)
				mapVal.Set(mapKey.MapKey(), mapValue)
			default:
				mapVal.Set(mapKey.MapKey(), u.unmarshalScalar(valueNode, mapValueField, false))
			}
		}
	}
}

func isNull(node *yaml.Node) bool {
	return node.Tag == "!!null"
}

// Resolve the node to be used with the custom unmarshaler. Returns nil if the
// there was an error.
func (u *unmarshaler) findNodeForCustom(node *yaml.Node, forAny bool) *yaml.Node {
	if !forAny {
		return node
	}
	if !u.checkKind(node, yaml.MappingNode) {
		return nil
	}
	var valueNode *yaml.Node
	for i := 1; i < len(node.Content); i += 2 {
		keyNode := node.Content[i-1]
		switch keyNode.Value {
		case "value":
			valueNode = node.Content[i]
		case atTypeFieldName:
			continue // Skip the @type field for Any messages
		default:
			u.addErrorf(keyNode, "unknown field %#v, expended one of %v", keyNode.Value, []string{"value", atTypeFieldName})
			return nil
		}
	}
	if valueNode == nil {
		u.addErrorf(node, "missing \"value\" field")
	}
	return valueNode
}

// Unmarshal the given yaml node into the given proto.Message.
func (u *unmarshaler) unmarshalMessage(node *yaml.Node, message proto.Message, forAny bool) {
	// Check for a custom unmarshaler

	if custom, ok := u.custom[message.ProtoReflect().Descriptor().FullName()]; ok {
		valueNode := u.findNodeForCustom(node, forAny)
		if valueNode == nil {
			return // Error already added.
		} else if custom(u, valueNode, message) {
			return // Custom unmarshaler handled the decoding.
		}
	}
	if isNull(node) {
		return // Null is always allowed for messages
	}
	if node.Kind != yaml.MappingNode {
		u.addErrorf(node, "expected fields for %v, got %v",
			message.ProtoReflect().Descriptor().FullName(), getNodeKind(node.Kind))
		return
	}
	// Decode the fields
	fields := message.ProtoReflect().Descriptor().Fields()
	for i := 0; i < len(node.Content); i += 2 {
		keyNode := node.Content[i]
		if u.checkKind(keyNode, yaml.ScalarNode) {
			if forAny && keyNode.Value == atTypeFieldName {
				continue // Skip the @type field for Any messages
			}
			// Get the field Name, JSONName, or Number
			if field := findField(keyNode.Value, fields); field != nil {
				valueNode := node.Content[i+1]
				u.unmarshalField(valueNode, field, message)
			} else {
				u.addErrorf(keyNode, "unknown field %#v, expended one of %v", keyNode.Value, getFieldNames(fields))
			}
		}
	}
}

type customUnmarshaler func(u *unmarshaler, node *yaml.Node, message proto.Message) bool

// Add all well-known type unmarshalers to the given map (including struct unmarshalers).
func addWktUnmarshalers(custom map[protoreflect.FullName]customUnmarshaler) {
	custom["google.protobuf.Any"] = unmarshalAnyMsg

	custom["google.protobuf.Duration"] = unmarshalDurationMsg
	custom["google.protobuf.Timestamp"] = unmarshalTimestampMsg

	custom["google.protobuf.BoolValue"] = unmarshalWrapperMsg
	custom["google.protobuf.BytesValue"] = unmarshalWrapperMsg
	custom["google.protobuf.DoubleValue"] = unmarshalWrapperMsg
	custom["google.protobuf.FloatValue"] = unmarshalWrapperMsg
	custom["google.protobuf.Int32Value"] = unmarshalWrapperMsg
	custom["google.protobuf.Int64Value"] = unmarshalWrapperMsg
	custom["google.protobuf.UInt32Value"] = unmarshalWrapperMsg
	custom["google.protobuf.UInt64Value"] = unmarshalWrapperMsg
	custom["google.protobuf.StringValue"] = unmarshalWrapperMsg

	custom["google.protobuf.Value"] = unmarshalValueMsg
	custom["google.protobuf.ListValue"] = unmarshalListValueMsg
	custom["google.protobuf.Struct"] = unmarshalStructMsg
}

func unmarshalAnyMsg(unm *unmarshaler, node *yaml.Node, message proto.Message) bool {
	if node.Kind != yaml.MappingNode || len(node.Content) == 0 {
		return false
	}
	anyVal, ok := message.(*anypb.Any)
	if !ok {
		anyVal = &anypb.Any{}
	}

	// Get the message type.
	msgType, err := unm.findAnyType(node)
	if err != nil {
		unm.addError(node, err)
		return true
	}

	protoVal := msgType.New()
	unm.unmarshalMessage(node, protoVal.Interface(), true)
	if err = anyVal.MarshalFrom(protoVal.Interface()); err != nil {
		unm.addErrorf(node, "failed to marshal %v: %v", msgType.Descriptor().FullName(), err)
	}

	if !ok {
		return setFieldByName(message, "type_url", protoreflect.ValueOfString(anyVal.GetTypeUrl())) &&
			setFieldByName(message, "value", protoreflect.ValueOfBytes(anyVal.GetValue()))
	}

	return true
}

const (
	maxTimestampSeconds = 253402300799
	minTimestampSeconds = -62135596800
)

// Format is decimal seconds with up to 9 fractional digits, followed by an 's'.
func parseDuration(txt string, duration *durationpb.Duration) error {
	// Remove trailing s.
	txt = strings.TrimSpace(txt)
	if len(txt) == 0 || txt[len(txt)-1] != 's' {
		return errors.New("missing trailing 's'")
	}
	value := txt[:len(txt)-1]
	isNeg := strings.HasPrefix(value, "-")

	// Split into seconds and nanos.
	parts := strings.Split(value, ".")
	switch len(parts) {
	case 1:
		// seconds only
		seconds, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return err
		}
		duration.Seconds = seconds
		duration.Nanos = 0
	case 2:
		// seconds and up to 9 digits of fractional seconds
		seconds, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			return err
		}
		duration.Seconds = seconds
		nanos, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil {
			return err
		}
		power := 9 - len(parts[1])
		if power < 0 {
			return errors.New("too many fractional second digits")
		}
		nanos *= int64(math.Pow10(power))
		if isNeg {
			duration.Nanos = -int32(nanos)
		} else {
			duration.Nanos = int32(nanos)
		}
	default:
		return errors.New("invalid duration: too many '.' characters")
	}
	return nil
}

// Format is RFC3339Nano, limited to the range 0001-01-01T00:00:00Z to
// 9999-12-31T23:59:59Z inclusive.
func parseTimestamp(txt string, timestamp *timestamppb.Timestamp) error {
	parsed, err := time.Parse(time.RFC3339Nano, txt)
	if err != nil {
		return err
	}
	// Validate seconds.
	secs := parsed.Unix()
	if secs < minTimestampSeconds {
		return errors.New("before 0001-01-01T00:00:00Z")
	} else if secs > maxTimestampSeconds {
		return errors.New("after 9999-12-31T23:59:59Z")
	}
	// Validate nanos.
	subsecond := strings.LastIndexByte(txt, '.')
	timezone := strings.LastIndexAny(txt, "Z-+")
	if subsecond >= 0 && timezone >= subsecond && timezone-subsecond > len(".999999999") {
		return errors.New("too many fractional second digits")
	}

	timestamp.Seconds = secs
	timestamp.Nanos = int32(parsed.Nanosecond())
	return nil
}

func setFieldByName(message proto.Message, name string, value protoreflect.Value) bool {
	field := message.ProtoReflect().Descriptor().Fields().ByName(protoreflect.Name(name))
	if field == nil {
		return false
	}
	message.ProtoReflect().Set(field, value)
	return true
}

func unmarshalDurationMsg(unm *unmarshaler, node *yaml.Node, message proto.Message) bool {
	if node.Kind != yaml.ScalarNode || len(node.Value) == 0 || isNull(node) {
		return false
	}
	duration, ok := message.(*durationpb.Duration)
	if !ok {
		duration = &durationpb.Duration{}
	}
	err := parseDuration(node.Value, duration)
	if err != nil {
		unm.addErrorf(node, "invalid duration: %v", err)
	} else if !ok {
		// Set the fields dynamically.
		return setFieldByName(message, "seconds", protoreflect.ValueOfInt64(duration.GetSeconds())) &&
			setFieldByName(message, "nanos", protoreflect.ValueOfInt32(duration.GetNanos()))
	}
	return true
}

func unmarshalTimestampMsg(unm *unmarshaler, node *yaml.Node, message proto.Message) bool {
	if node.Kind != yaml.ScalarNode || len(node.Value) == 0 || isNull(node) {
		return false
	}
	timestamp, ok := message.(*timestamppb.Timestamp)
	if !ok {
		timestamp = &timestamppb.Timestamp{}
	}
	err := parseTimestamp(node.Value, timestamp)
	if err != nil {
		unm.addErrorf(node, "invalid timestamp: %v", err)
	} else if !ok {
		return setFieldByName(message, "seconds", protoreflect.ValueOfInt64(timestamp.GetSeconds())) &&
			setFieldByName(message, "nanos", protoreflect.ValueOfInt32(timestamp.GetNanos()))
	}
	return true
}

// Forwards unmarshaling to the "value" field of the given wrapper message.
func unmarshalWrapperMsg(unm *unmarshaler, node *yaml.Node, message proto.Message) bool {
	valueField := message.ProtoReflect().Descriptor().Fields().ByName("value")
	if node.Kind == yaml.MappingNode || valueField == nil {
		return false
	}
	unm.unmarshalField(node, valueField, message)
	return true
}

func dynSetValue(message proto.Message, value *structpb.Value) bool {
	switch val := value.GetKind().(type) {
	case *structpb.Value_NullValue:
		return setFieldByName(message, "null_value", protoreflect.ValueOfEnum(protoreflect.EnumNumber(val.NullValue)))
	case *structpb.Value_NumberValue:
		return setFieldByName(message, "number_value", protoreflect.ValueOfFloat64(val.NumberValue))
	case *structpb.Value_StringValue:
		return setFieldByName(message, "string_value", protoreflect.ValueOfString(val.StringValue))
	case *structpb.Value_BoolValue:
		return setFieldByName(message, "bool_value", protoreflect.ValueOfBool(val.BoolValue))
	case *structpb.Value_ListValue:
		listFld := message.ProtoReflect().Descriptor().Fields().ByName("list_value")
		if listFld == nil {
			return false
		}
		listVal := message.ProtoReflect().Mutable(listFld).Message().Interface()
		return dynSetListValue(listVal, val.ListValue)
	case *structpb.Value_StructValue:
		structFld := message.ProtoReflect().Descriptor().Fields().ByName("struct_value")
		if structFld == nil {
			return false
		}
		structVal := message.ProtoReflect().Mutable(structFld).Message().Interface()
		return dynSetStruct(structVal, val.StructValue)
	}
	return false
}

func dynSetListValue(message proto.Message, list *structpb.ListValue) bool {
	valuesFld := message.ProtoReflect().Descriptor().Fields().ByName("values")
	if valuesFld == nil {
		return false
	}
	values := message.ProtoReflect().Mutable(valuesFld).List()
	for _, item := range list.GetValues() {
		value := values.NewElement()
		if !dynSetValue(value.Message().Interface(), item) {
			return false
		}
		values.Append(value)
	}
	return true
}

func dynSetStruct(message proto.Message, structVal *structpb.Struct) bool {
	fieldsFld := message.ProtoReflect().Descriptor().Fields().ByName("fields")
	if fieldsFld == nil {
		return false
	}
	fields := message.ProtoReflect().Mutable(fieldsFld).Map()
	for key, item := range structVal.GetFields() {
		value := fields.NewValue()
		if !dynSetValue(value.Message().Interface(), item) {
			return false
		}
		fields.Set(protoreflect.ValueOfString(key).MapKey(), value)
	}
	return true
}

func unmarshalValueMsg(unm *unmarshaler, node *yaml.Node, message proto.Message) bool {
	value, ok := message.(*structpb.Value)
	if !ok {
		value = &structpb.Value{}
	}
	unm.unmarshalValue(node, value)
	if !ok {
		return dynSetValue(message, value)
	}
	return true
}

func unmarshalListValueMsg(unm *unmarshaler, node *yaml.Node, message proto.Message) bool {
	if node.Kind != yaml.SequenceNode {
		return false
	}
	listValue, ok := message.(*structpb.ListValue)
	if !ok {
		listValue = &structpb.ListValue{}
	}
	unm.unmarshalListValue(node, listValue)
	if !ok {
		return dynSetListValue(message, listValue)
	}
	return true
}

func unmarshalStructMsg(unm *unmarshaler, node *yaml.Node, message proto.Message) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}
	structVal, ok := message.(*structpb.Struct)
	if !ok {
		structVal = &structpb.Struct{}
	}
	unm.unmarshalStruct(node, structVal)
	if !ok {
		return dynSetStruct(message, structVal)
	}
	return true
}

// Unmarshal the given yaml node into a structpb.Value, using the given
// field descriptor to validate the type, if non-nil.
func (u *unmarshaler) unmarshalValue(
	node *yaml.Node,
	value *structpb.Value,
) {
	// Unmarshal the value.
	switch node.Kind {
	case yaml.SequenceNode: // A list.
		listValue := &structpb.ListValue{}
		u.unmarshalListValue(node, listValue)
		value.Kind = &structpb.Value_ListValue{ListValue: listValue}
	case yaml.MappingNode: // A message or map.
		structVal := &structpb.Struct{}
		u.unmarshalStruct(node, structVal)
		value.Kind = &structpb.Value_StructValue{StructValue: structVal}
	case yaml.ScalarNode:
		u.unmarshalScalarValue(node, value)
	case 0:
		value.Kind = &structpb.Value_NullValue{}
	default:
		u.addErrorf(node, "unimplemented value kind: %v", getNodeKind(node.Kind))
	}
}

// Unmarshal the given yaml node into a structpb.ListValue, using the given field
// descriptor to validate each item, if non-nil.
func (u *unmarshaler) unmarshalListValue(
	node *yaml.Node,
	list *structpb.ListValue,
) {
	for _, itemNode := range node.Content {
		itemValue := &structpb.Value{}
		u.unmarshalValue(itemNode, itemValue)
		list.Values = append(list.GetValues(), itemValue)
	}
}

// Unmarshal the given yaml node into a structpb.Struct
//
// Structs can represent either a message or a map.
// For messages, the message descriptor can be provided or inferred from the node.
// For maps, the field descriptor can be provided to validate the map keys/values.
func (u *unmarshaler) unmarshalStruct(
	node *yaml.Node,
	message *structpb.Struct,
) {
	for i := 1; i < len(node.Content); i += 2 {
		keyNode := node.Content[i-1]
		// Validate the key.
		if !u.checkKind(keyNode, yaml.ScalarNode) {
			continue
		}

		// Unmarshal the value.
		valueNode := node.Content[i]
		value := &structpb.Value{}
		u.unmarshalValue(valueNode, value)
		if message.GetFields() == nil {
			message.Fields = make(map[string]*structpb.Value)
		}
		message.Fields[keyNode.Value] = value
	}
}

func (u *unmarshaler) unmarshalScalarValue(node *yaml.Node, value *structpb.Value) {
	switch node.Tag {
	case "!!null":
		value.Kind = &structpb.Value_NullValue{}
	case "!!bool":
		u.unmarshalScalarBool(node, value)
	default:
		u.unmarshalScalarString(node, value)
	}
}

// bool, string, or bytes.
func (u *unmarshaler) unmarshalScalarBool(node *yaml.Node, value *structpb.Value) {
	switch node.Value {
	case "true":
		value.Kind = &structpb.Value_BoolValue{BoolValue: true}
	case "false":
		value.Kind = &structpb.Value_BoolValue{BoolValue: false}
	default: // This is a string, not a bool.
		value.Kind = &structpb.Value_StringValue{StringValue: node.Value}
	}
}

// Can be string, bytes, float, or int.
func (u *unmarshaler) unmarshalScalarString(node *yaml.Node, value *structpb.Value) {
	floatVal, err := strconv.ParseFloat(node.Value, 64)
	if err != nil {
		value.Kind = &structpb.Value_StringValue{StringValue: node.Value}
		return
	}

	if math.IsInf(floatVal, 0) || math.IsNaN(floatVal) {
		// String or float.
		value.Kind = &structpb.Value_StringValue{StringValue: node.Value}
		return
	}

	// String, float, or int.
	u.unmarshalScalarFloat(node, value, floatVal)
}

func (u *unmarshaler) unmarshalScalarFloat(node *yaml.Node, value *structpb.Value, floatVal float64) {
	// Try to parse it as in integer, to see if the float representation is lossy.
	lit, litErr := parseIntLiteral(node.Value)

	// Check if we can represent this as a number.
	floatUintVal := uint64(math.Abs(floatVal))      // The uint64 representation of the float.
	if litErr != nil || floatUintVal == lit.value { // Safe to represent as a number.
		value.Kind = &structpb.Value_NumberValue{NumberValue: floatVal}
	} else { // Keep string representation.
		value.Kind = &structpb.Value_StringValue{StringValue: node.Value}
	}
}

// NodeClosestToPath returns the node closest to the given field path.
//
// If toKey is true, the key node is returned if the path points to a map entry.
//
// Example field paths:
//   - 'foo' -> the field foo
//   - 'foo[0]' -> the first element of the repeated field foo or the map entry with key '0'
//   - 'foo.bar' -> the field bar in the message field foo
//   - 'foo["bar"]' -> the map entry with key 'bar' in the map field foo
func nodeClosestToPath(root *yaml.Node, msgDesc protoreflect.MessageDescriptor, path string, toKey bool) *yaml.Node {
	parsedPath, err := parseFieldPath(path)
	if err != nil {
		return root
	}
	return findNodeByPath(root, msgDesc, parsedPath, toKey)
}

func parseFieldPath(path string) ([]string, error) {
	if len(path) == 0 {
		return nil, nil
	}
	next, path := parseNextFieldName(path)
	result := []string{next}
	for len(path) > 0 {
		switch path[0] {
		case '[': // Parse array index or map key.
			next, path = parseNextValue(path[1:])
		case '.': // Parse field name.
			next, path = parseNextFieldName(path[1:])
		default:
			return nil, errors.New("invalid path")
		}
		result = append(result, next)
	}
	return result, nil
}

func parseNextFieldName(path string) (string, string) {
	for i := 0; i < len(path); i++ {
		switch path[i] {
		case '.':
			return path[:i], path[i:]
		case '[':
			return path[:i], path[i:]
		}
	}
	return path, ""
}

func parseNextValue(path string) (string, string) {
	if len(path) == 0 {
		return "", ""
	}
	if path[0] == '"' {
		// Parse string.
		for i := 1; i < len(path); i++ {
			if path[i] == '\\' {
				i++ // Skip escaped character.
			} else if path[i] == '"' {
				result, err := strconv.Unquote(path[:i+1])
				if err != nil {
					return "", ""
				}
				return result, path[i+2:]
			}
		}
		return path, ""
	}
	// Go til the trailing ']'
	for i := 0; i < len(path); i++ {
		if path[i] == ']' {
			return path[:i], path[i+1:]
		}
	}
	return path, ""
}

// Returns the node as close to the given path as possible.
func findNodeByPath(root *yaml.Node, msgDesc protoreflect.MessageDescriptor, path []string, toKey bool) *yaml.Node {
	cur := root
	curMsg := msgDesc
	var curMap protoreflect.FieldDescriptor
	for i, key := range path {
		switch cur.Kind {
		case yaml.MappingNode:
			if curMsg != nil {
				field := findField(key, curMsg.Fields())
				if field == nil {
					return cur
				}
				var found bool
				cur, found = findNodeByField(cur, field)
				switch {
				case !found:
					return cur
				case field.IsMap():
					curMap = field
					curMsg = nil
				default:
					curMap = nil
					curMsg = field.Message()
				}
			} else if curMap != nil {
				var found bool
				var keyNode *yaml.Node
				keyNode, cur, found = findEntryByKey(cur, key)
				if !found {
					return cur
				}
				if i == len(path)-1 && toKey {
					return keyNode
				}
				curMsg = curMap.MapValue().Message()
				curMap = nil
			}
		case yaml.SequenceNode:
			idx, err := strconv.Atoi(key)
			if err != nil || idx < 0 || idx >= len(cur.Content) {
				return cur
			}
			cur = cur.Content[idx]
		default:
			return cur
		}
	}
	return cur
}

func findNodeByField(cur *yaml.Node, field protoreflect.FieldDescriptor) (*yaml.Node, bool) {
	fieldNum := fmt.Sprintf("%d", field.Number())
	for i := 1; i < len(cur.Content); i += 2 {
		keyNode := cur.Content[i-1]
		if keyNode.Value == string(field.Name()) ||
			keyNode.Value == field.JSONName() ||
			keyNode.Value == fieldNum {
			return cur.Content[i], true
		}
	}
	return cur, false
}

func findEntryByKey(cur *yaml.Node, key string) (*yaml.Node, *yaml.Node, bool) {
	for i := 1; i < len(cur.Content); i += 2 {
		keyNode := cur.Content[i-1]
		if keyNode.Value == key {
			return keyNode, cur.Content[i], true
		}
	}
	return nil, cur, false
}
