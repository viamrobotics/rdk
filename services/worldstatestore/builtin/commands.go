package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	commonpb "go.viam.com/api/common/v1"
	pb "go.viam.com/api/service/worldstatestore/v1"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/worldstatestore"
)

// The DoCommands that change the store. Each takes a list of transforms, so many can be changed
// at once. A transform is written in its proto JSON form, in which a uuid is base64 encoded.
//
// One call can carry any combination of the three commands. They are applied in the order
// remove, add, update, whatever order they were written in, so that a call can replace a
// transform by removing it and adding it again under the same uuid.
//
// A call applies everything it carries or nothing. If any item is invalid the store is left
// untouched and the error names the command and the item's index.
//
// The response holds, for each command the call carried, the base64 uuid of each transform that
// command changed, in the order of its list:
//
//	{"add": ["Ym94"], "remove": ["b2xk"]}
const (
	// DoAdd adds transforms. Each item is a whole transform:
	//
	//	{"add": [{"uuid": "Ym94", "referenceFrame": "box", "poseInObserverFrame": {...}}]}
	//
	// A transform needs a reference frame and an observer frame. One without a uuid is given a
	// random one, which the response reports. Adding a uuid that is already stored is an error.
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

	updatedFieldsKey = "updatedFields"
)

// commandOrder is the order in which the commands of one call are applied.
var commandOrder = []string{DoRemove, DoAdd, DoUpdate}

// DoCommand applies the `DoAdd`, `DoUpdate` and `DoRemove` commands that `cmd` carries.
func (svc *builtIn) DoCommand(ctx context.Context, cmd map[string]any) (map[string]any, error) {
	decoded := make(map[string][]*pb.StreamTransformChangesResponse, len(commandOrder))
	for _, name := range commandOrder {
		raw, ok := cmd[name]
		if !ok {
			continue
		}
		events, err := decodeEvents(name, raw)
		if err != nil {
			return nil, err
		}
		decoded[name] = events
	}
	if len(decoded) == 0 {
		return nil, resource.ErrDoUnimplemented
	}

	svc.mu.Lock()
	defer svc.mu.Unlock()

	if svc.closed {
		return nil, errClosed
	}

	staged := newBatch(svc.transforms)
	ret := make(map[string]any, len(decoded))
	for _, name := range commandOrder {
		events, ok := decoded[name]
		if !ok {
			continue
		}

		firstChange := len(staged.changes)
		var err error
		switch name {
		case DoRemove:
			err = staged.remove(events)
		case DoAdd:
			err = staged.add(DoAdd, events)
		case DoUpdate:
			err = staged.update(events)
		}
		if err != nil {
			return nil, err
		}

		uuids := make([]any, 0, len(events))
		for _, change := range staged.changes[firstChange:] {
			uuids = append(uuids, base64.StdEncoding.EncodeToString(change.Transform.GetUuid()))
		}
		ret[name] = uuids
	}

	svc.commitLocked(staged)
	svc.logger.CDebugw(ctx, "Applied world state changes", "numChanges", len(staged.changes))
	return ret, nil
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

// batch stages changes against the store, so that everything one call carries can be checked
// before any of it is applied. A change sees the effect of the changes staged before it. Nothing
// reaches the store until `commitLocked`, so a batch that hits an invalid item is simply dropped.
type batch struct {
	// stored is the store's own map. A batch only reads it.
	stored map[string]*commonpb.Transform
	// staged holds what the batch has added or updated, and nil for what it has removed.
	staged map[string]*commonpb.Transform
	// changes are the events to publish if the batch is committed, in the order they were staged.
	changes []worldstatestore.TransformChange
}

func newBatch(stored map[string]*commonpb.Transform) *batch {
	return &batch{stored: stored, staged: make(map[string]*commonpb.Transform)}
}

// get returns the transform as it would be if the batch were committed now.
func (b *batch) get(key string) (*commonpb.Transform, bool) {
	if transform, ok := b.staged[key]; ok {
		return transform, transform != nil
	}
	transform, ok := b.stored[key]
	return transform, ok
}

// add stages new transforms. `label` names the list in errors: the command, or the config key.
func (b *batch) add(label string, events []*pb.StreamTransformChangesResponse) error {
	for idx, event := range events {
		transform := event.GetTransform()
		if transform.GetReferenceFrame() == "" {
			return fmt.Errorf("%s[%d]: transform has no reference frame", label, idx)
		}
		if transform.GetPoseInObserverFrame().GetReferenceFrame() == "" {
			return fmt.Errorf("%s[%d]: transform has no observer reference frame", label, idx)
		}
		if len(transform.GetUuid()) == 0 {
			id := uuid.New()
			transform.Uuid = id[:]
		}

		key := string(transform.GetUuid())
		if _, exists := b.get(key); exists {
			return fmt.Errorf("%s[%d]: transform %q already exists", label, idx, key)
		}

		b.staged[key] = transform
		b.changes = append(b.changes, worldstatestore.TransformChange{
			ChangeType: pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_ADDED,
			Transform:  transform,
		})
	}
	return nil
}

// update stages changes to fields of existing transforms. The same transform can be updated more
// than once, each update building on the one before.
func (b *batch) update(events []*pb.StreamTransformChangesResponse) error {
	for idx, event := range events {
		key := string(event.GetTransform().GetUuid())
		current, exists := b.get(key)
		if !exists {
			return fmt.Errorf("%s[%d]: transform %q not found", DoUpdate, idx, key)
		}

		merged, changed, paths, err := mergeTransform(current, event.GetTransform(), event.GetUpdatedFields().GetPaths())
		if err != nil {
			return fmt.Errorf("%s[%d]: %w", DoUpdate, idx, err)
		}

		b.staged[key] = merged
		b.changes = append(b.changes, worldstatestore.TransformChange{
			ChangeType:    pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_UPDATED,
			Transform:     changed,
			UpdatedFields: paths,
		})
	}
	return nil
}

// remove stages the removal of existing transforms.
func (b *batch) remove(events []*pb.StreamTransformChangesResponse) error {
	for idx, event := range events {
		key := string(event.GetTransform().GetUuid())
		if _, exists := b.get(key); !exists {
			return fmt.Errorf("%s[%d]: transform %q not found", DoRemove, idx, key)
		}

		b.staged[key] = nil
		// The world state store API specifies that a removal's field mask is the uuid path.
		b.changes = append(b.changes, worldstatestore.TransformChange{
			ChangeType:    pb.TransformChangeType_TRANSFORM_CHANGE_TYPE_REMOVED,
			Transform:     &commonpb.Transform{Uuid: []byte(key)},
			UpdatedFields: []string{uuidField},
		})
	}
	return nil
}

// commitLocked applies a batch to the store and publishes its changes.
func (svc *builtIn) commitLocked(staged *batch) {
	for key, transform := range staged.staged {
		if transform == nil {
			delete(svc.transforms, key)
			continue
		}
		svc.transforms[key] = transform
	}
	svc.publishLocked(staged.changes)
}
