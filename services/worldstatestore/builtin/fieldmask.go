package builtin

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const uuidField = "uuid"

// mergeTransform returns a copy of `stored` with the fields named by `paths` taken from `patch`.
// A named field that `patch` leaves unset is cleared.
//
// `stored` is left as it is, and the copy shares every part of it that the paths do not reach.
// An update therefore costs what it touches, not the size of the transform: moving a transform
// does not copy its point cloud.
//
// Paths are dot-separated field names, in either their proto (`pose_in_observer_frame.pose.x`)
// or JSON (`poseInObserverFrame.pose.x`) spelling. A path can name a whole message, which is
// replaced, but cannot reach inside a list or map.
//
// It also returns the event to publish for the update, and the paths in their proto spelling.
// The event carries the complete merged value of every top-level field that was touched, rather
// than only the named leaves. Consumers that apply an update one top-level field at a time
// would otherwise reset the rest of that field.
func mergeTransform(stored, patch *commonpb.Transform, paths []string) (
	merged, event *commonpb.Transform, protoPaths []string, err error,
) {
	if len(paths) == 0 {
		return nil, nil, nil, errors.New("an update must change at least one field")
	}

	merged = shallowCopy(stored.ProtoReflect()).Interface().(*commonpb.Transform)
	touched := make([]protoreflect.FieldDescriptor, 0, len(paths))
	protoPaths = make([]string, 0, len(paths))
	for _, path := range paths {
		topLevel, protoPath, err := mergePath(merged.ProtoReflect(), patch.ProtoReflect(), path)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("path %q: %w", path, err)
		}
		touched = append(touched, topLevel)
		protoPaths = append(protoPaths, protoPath)
	}

	event = &commonpb.Transform{Uuid: stored.GetUuid()}
	for _, fd := range touched {
		copyField(event.ProtoReflect(), merged.ProtoReflect(), fd)
	}
	return merged, event, protoPaths, nil
}

// inferPaths returns the paths of the fields that a JSON transform spells out, so that an update
// changes exactly what its sender typed. This has to read the JSON: once decoded, a field sent
// as zero looks the same as one that was not sent.
//
// It descends into nested messages and stops at anything `mergePath` replaces whole: scalars,
// lists, and messages with their own JSON form such as the metadata struct. A field sent as
// null or as an empty object is itself a path, which clears or empties it. The uuid identifies
// the transform and is skipped.
func inferPaths(desc protoreflect.MessageDescriptor, object map[string]json.RawMessage, prefix string) ([]string, error) {
	var ret []string
	for _, key := range slices.Sorted(maps.Keys(object)) {
		fd, err := fieldNamed(desc, key)
		if err != nil {
			return nil, err
		}
		if prefix == "" && fd.Name() == uuidField {
			continue
		}
		path := prefix + string(fd.Name())

		hasFieldwiseJSON := fd.Message() != nil && !fd.IsList() && !fd.IsMap() &&
			fd.Message().ParentFile().Package() != "google.protobuf"
		var nested map[string]json.RawMessage
		if hasFieldwiseJSON && json.Unmarshal(object[key], &nested) == nil && len(nested) > 0 {
			nestedPaths, err := inferPaths(fd.Message(), nested, path+".")
			if err != nil {
				return nil, err
			}
			ret = append(ret, nestedPaths...)
			continue
		}
		ret = append(ret, path)
	}
	return ret, nil
}

// mergePath copies the field at `path` from `src` to `dst`. It returns the descriptor of the
// path's first field and the path in its proto spelling.
func mergePath(dst, src protoreflect.Message, path string) (protoreflect.FieldDescriptor, string, error) {
	segments := strings.Split(path, ".")
	protoSegments := make([]string, 0, len(segments))
	var topLevel protoreflect.FieldDescriptor

	for idx, segment := range segments {
		fd, err := fieldNamed(dst.Descriptor(), segment)
		if err != nil {
			return nil, "", err
		}
		protoSegments = append(protoSegments, string(fd.Name()))

		if idx == 0 {
			if fd.Name() == uuidField {
				return nil, "", errors.New("the uuid of a transform cannot be updated")
			}
			topLevel = fd
		}

		if idx == len(segments)-1 {
			copyField(dst, src, fd)
			break
		}

		if fd.Message() == nil || fd.IsList() || fd.IsMap() {
			return nil, "", fmt.Errorf("cannot address a field inside %q", segment)
		}
		// The message at `fd` may still be shared with the stored transform, so it is replaced
		// by a copy before anything inside it changes. `Get` on an unset message field returns an
		// empty read-only message, so this also creates a parent that `dst` lacks, and a patch
		// that omits a parent clears the named leaf.
		child := shallowCopy(dst.Get(fd).Message())
		dst.Set(fd, protoreflect.ValueOfMessage(child))
		src = src.Get(fd).Message()
		dst = child
	}

	return topLevel, strings.Join(protoSegments, "."), nil
}

// fieldNamed returns the field of a message with the given name, in its proto or JSON spelling.
func fieldNamed(desc protoreflect.MessageDescriptor, name string) (protoreflect.FieldDescriptor, error) {
	fields := desc.Fields()
	if fd := fields.ByName(protoreflect.Name(name)); fd != nil {
		return fd, nil
	}
	if fd := fields.ByJSONName(name); fd != nil {
		return fd, nil
	}
	return nil, fmt.Errorf("%s has no field %q", desc.Name(), name)
}

// shallowCopy returns a new message whose fields are those of `msg`. The two share the memory
// of every field, so the copy's fields can be replaced but must not be changed in place.
func shallowCopy(msg protoreflect.Message) protoreflect.Message {
	ret := msg.New()
	msg.Range(func(fd protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		ret.Set(fd, value)
		return true
	})
	ret.SetUnknown(msg.GetUnknown())
	return ret
}

// copyField makes `fd` in `dst` equal to `fd` in `src`. The two then share the field's memory,
// which is safe for both uses here: an update's patch is discarded once it has been merged, and
// an event is read-only like the stored transform it is built from.
func copyField(dst, src protoreflect.Message, fd protoreflect.FieldDescriptor) {
	if !src.Has(fd) {
		dst.Clear(fd)
		return
	}
	dst.Set(fd, src.Get(fd))
}
