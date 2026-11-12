package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"

	"github.com/google/uuid"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// The DoCommands that change the store. Each takes a list of transforms, so many can be changed
// in one call. A transform is written in its proto JSON form, in which a uuid is base64 encoded.
//
// A call applies its whole list or none of it. If any item is invalid the store is left
// untouched and the error names the item's index.
const (
	// DoAdd adds transforms. Each item is a whole transform:
	//
	//	{"add": [{"uuid": "Ym94", "referenceFrame": "box", "poseInObserverFrame": {...}}]}
	//
	// A transform needs a reference frame and an observer frame. One without a uuid is given a
	// random one. Adding a uuid that is already stored is an error.
	DoAdd = "add"

	// DoUpdate changes fields of stored transforms. Each item is a transform holding the uuid and
	// only the fields to change. This moves the transform along x and leaves the rest of its
	// pose alone:
	//
	//	{"update": [{"uuid": "Ym94", "poseInObserverFrame": {"pose": {"x": 5}}}]}
	//
	// A field sent as null is cleared. An item can instead say which fields to take from it with
	// an `updatedFields` list of paths, such as `["poseInObserverFrame.pose.x"]`; then anything
	// else it holds is ignored.
	DoUpdate = "update"

	// DoRemove removes transforms. Each item only needs the transform's uuid:
	//
	//	{"remove": [{"uuid": "Ym94"}]}
	DoRemove = "remove"

	// UUIDsKey is the key of the response to every command above. It holds the base64 uuid of
	// each transform the call changed, in the order of its list.
	UUIDsKey = "uuids"

	updatedFieldsKey = "updatedFields"
)

// DoCommand applies one of `DoAdd`, `DoUpdate` or `DoRemove`.
func (svc *builtIn) DoCommand(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	var name string
	for _, candidate := range []string{DoAdd, DoUpdate, DoRemove} {
		if _, ok := cmd[candidate]; !ok {
			continue
		}
		if name != "" {
			return nil, fmt.Errorf("only one of %q, %q and %q can be sent per command", DoAdd, DoUpdate, DoRemove)
		}
		name = candidate
	}
	if name == "" {
		return nil, resource.ErrDoUnimplemented
	}

	events, err := decodeEvents(name, cmd[name])
	if err != nil {
		return nil, err
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.closed {
		return nil, errClosed
	}

	var changes []worldstatestore.TransformChange
	switch name {
	case DoAdd:
		changes, err = svc.addLocked(events)
	case DoUpdate:
		changes, err = svc.updateLocked(events)
	case DoRemove:
		changes, err = svc.removeLocked(events)
	}
	if err != nil {
		return nil, err
	}

	uuids := make([]any, 0, len(changes))
	for _, change := range changes {
		svc.publishLocked(change)
		uuids = append(uuids, base64.StdEncoding.EncodeToString(change.Transform.GetUuid()))
	}
	svc.logger.CDebugw(ctx, "Applied world state changes", "command", name, "numTransforms", len(changes))

	return map[string]any{UUIDsKey: uuids}, nil
}

// decodeEvents parses the list of transforms sent with the command `name`. Each is returned as
// the change it asks for: the transform, and for an update the paths of its fields to apply.
func decodeEvents(name string, raw any) ([]*pb.StreamTransformChangesResponse, error) {
	// Round-tripping through JSON accepts whatever slice type the caller built, as well as the
	// `[]any` a command has after crossing gRPC.
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	var items []json.RawMessage
	if err := json.Unmarshal(encoded, &items); err != nil {
		return nil, fmt.Errorf("%s: expected a list of transforms: %w", name, err)
	}

	ret := make([]*pb.StreamTransformChangesResponse, 0, len(items))
	for idx, item := range items {
		event, err := decodeEvent(item, name == DoUpdate)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, idx, err)
		}
		ret = append(ret, event)
	}
	return ret, nil
}

// decodeEvent parses one list item: a proto JSON transform that may also carry `updatedFields`,
// a list of field paths. If it does not and `inferUpdatedFields` is set, the paths are the fields
// the item spells out.
func decodeEvent(item json.RawMessage, inferUpdatedFields bool) (*pb.StreamTransformChangesResponse, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(item, &fields); err != nil {
		return nil, fmt.Errorf("expected a transform object: %w", err)
	}

	var paths []string
	for _, key := range []string{updatedFieldsKey, "updated_fields"} {
		rawPaths, ok := fields[key]
		if !ok {
			continue
		}
		if err := json.Unmarshal(rawPaths, &paths); err != nil {
			return nil, fmt.Errorf("%s must be a list of field paths: %w", key, err)
		}
		delete(fields, key)
	}

	rest, err := json.Marshal(fields)
	if err != nil {
		return nil, err
	}
	transform := &commonpb.Transform{}
	if err := protojson.Unmarshal(rest, transform); err != nil {
		return nil, err
	}
	if inferUpdatedFields && paths == nil {
		if paths, err = inferPaths(transform.ProtoReflect().Descriptor(), fields, ""); err != nil {
			return nil, err
		}
	}
	event := &pb.StreamTransformChangesResponse{Transform: transform}
	if len(paths) > 0 {
		event.UpdatedFields = &fieldmaskpb.FieldMask{Paths: paths}
	}
	return event, nil
}

// The three functions below each validate a whole batch against a staged view of the store
// before touching `svc.transforms`, so a batch with an invalid item changes nothing. They
// return the changes to publish.

func (svc *builtIn) addLocked(events []*pb.StreamTransformChangesResponse) ([]worldstatestore.TransformChange, error) {
	staged := make(map[string]*commonpb.Transform, len(events))
	changes := make([]worldstatestore.TransformChange, 0, len(events))
	for idx, event := range events {
		transform := event.GetTransform()
		if transform.GetReferenceFrame() == "" {
			return nil, fmt.Errorf("%s[%d]: transform has no reference frame", DoAdd, idx)
		}
		if transform.GetPoseInObserverFrame().GetReferenceFrame() == "" {
			return nil, fmt.Errorf("%s[%d]: transform has no observer reference frame", DoAdd, idx)
		}
		if len(transform.GetUuid()) == 0 {
			id := uuid.New()
			transform.Uuid = id[:]
		}

		key := string(transform.GetUuid())
		_, isStored := svc.transforms[key]
		_, isStaged := staged[key]
		if isStored || isStaged {
			return nil, fmt.Errorf("%s[%d]: transform %q already exists", DoAdd, idx, key)
		}

		staged[key] = transform
		changes = append(changes, worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  transform,
		})
	}

	maps.Copy(svc.transforms, staged)
	return changes, nil
}

func (svc *builtIn) updateLocked(events []*pb.StreamTransformChangesResponse) ([]worldstatestore.TransformChange, error) {
	// A batch can update the same transform more than once, so later events merge into the
	// staged result of earlier ones.
	staged := make(map[string]*commonpb.Transform, len(events))
	changes := make([]worldstatestore.TransformChange, 0, len(events))
	for idx, event := range events {
		key := string(event.GetTransform().GetUuid())
		current, ok := staged[key]
		if !ok {
			current, ok = svc.transforms[key]
		}
		if !ok {
			return nil, fmt.Errorf("%s[%d]: transform %q not found", DoUpdate, idx, key)
		}

		merged, changed, paths, err := mergeTransform(current, event.GetTransform(), event.GetUpdatedFields().GetPaths())
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", DoUpdate, idx, err)
		}

		staged[key] = merged
		changes = append(changes, worldstatestore.TransformChange{
			ChangeType:    pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
			Transform:     changed,
			UpdatedFields: paths,
		})
	}

	maps.Copy(svc.transforms, staged)
	return changes, nil
}

func (svc *builtIn) removeLocked(events []*pb.StreamTransformChangesResponse) ([]worldstatestore.TransformChange, error) {
	staged := make(map[string]struct{}, len(events))
	changes := make([]worldstatestore.TransformChange, 0, len(events))
	for idx, event := range events {
		key := string(event.GetTransform().GetUuid())
		_, isStored := svc.transforms[key]
		_, isStaged := staged[key]
		if !isStored || isStaged {
			return nil, fmt.Errorf("%s[%d]: transform %q not found", DoRemove, idx, key)
		}

		staged[key] = struct{}{}
		// The world state store API specifies that a removal's field mask is the uuid path.
		changes = append(changes, worldstatestore.TransformChange{
			ChangeType:    pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
			Transform:     &commonpb.Transform{Uuid: []byte(key)},
			UpdatedFields: []string{uuidField},
		})
	}

	for key := range staged {
		delete(svc.transforms, key)
	}
	return changes, nil
}
