package builtin

import (
	"context"
	"fmt"
	"strings"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/services/worldstatestore"
	"go.viam.com/rdk/spatialmath"
)

// This file feeds the configured world state stores into planning. Every `Move` reads the stores
// afresh and plans around what they hold, in addition to any world state the request carries.
//
// A problem with a store never fails a `Move`. Whatever cannot be used is logged and skipped,
// because planning with the obstacles that could be read is better than refusing to move.
//
// This is deliberately naive: the stores are read one after another, with one `GetTransform` call
// per transform, on every `Move`. Nothing is cached, and only `Move` consults the stores. Each of
// those is a known follow-up rather than an oversight.

// The names used in log lines for where a piece of world state came from.
const (
	fromRequest = "request"
	fromStore   = "world state store"
)

// withStoreWorldState returns the request's world state combined with what the configured world
// state stores hold. Without stores, or with empty ones, it returns the request's world state
// as is.
func (ms *builtIn) withStoreWorldState(ctx context.Context, request *referenceframe.WorldState) *referenceframe.WorldState {
	if len(ms.worldStateStores) == 0 {
		return request
	}

	obstacles, transforms := ms.storeWorldState(ctx, request)
	ms.logger.CDebugw(ctx, "Read the world state stores for a move",
		"numStores", len(ms.worldStateStores),
		"numObstacles", len(obstacles),
		"numTransforms", len(transforms),
		"obstacles", describeObstacles(obstacles),
	)
	return ms.mergeWorldState(ctx, request, obstacles, transforms)
}

// storeWorldState reads the world state stores. A stored transform with a geometry becomes an
// obstacle. One without a geometry, or one marked visual only, becomes a supplemental frame that
// other transforms can be parented to.
//
// A frame name has one definition: the robot's, else the request's, else that of the first store
// to use it. A later definition is skipped, and what is parented to that name attaches to the
// definition that stood. Obstacle labels are not names in that sense, and are made unique later.
func (ms *builtIn) storeWorldState(
	ctx context.Context, request *referenceframe.WorldState,
) ([]*referenceframe.GeometriesInFrame, []*referenceframe.LinkInFrame) {
	knownFrames := ms.frameNames(ctx)
	for _, link := range request.Transforms() {
		knownFrames[link.Name()] = true
	}

	var obstacles []*referenceframe.GeometriesInFrame
	var transforms []*referenceframe.LinkInFrame
	for _, store := range ms.worldStateStores {
		storeName := store.Name().Name
		storeObstacles, storeTransforms := ms.placeStoreLinks(ctx, storeName, ms.readStore(ctx, store), knownFrames)
		obstacles = append(obstacles, storeObstacles...)
		transforms = append(transforms, storeTransforms...)
	}
	return obstacles, transforms
}

// readStore returns the transforms of one store as links. The link of a transform that is marked
// visual only has no geometry.
func (ms *builtIn) readStore(ctx context.Context, store worldstatestore.Service) []*referenceframe.LinkInFrame {
	storeName := store.Name().Name
	uuids, err := store.ListUUIDs(ctx, nil)
	if err != nil {
		ms.logger.CWarnw(ctx, "Failed to list a world state store's transforms. Planning without it.",
			"store", storeName, "err", err)
		return nil
	}

	ret := make([]*referenceframe.LinkInFrame, 0, len(uuids))
	for _, uuid := range uuids {
		transform, err := store.GetTransform(ctx, uuid, nil)
		if err != nil {
			ms.logger.CWarnw(ctx, "Failed to get a transform from a world state store. Skipping it.",
				"store", storeName, "uuid", fmt.Sprintf("%q", uuid), "err", err)
			continue
		}
		if transform == nil {
			continue
		}

		link, err := referenceframe.LinkInFrameFromTransformProtobuf(withGeometryCenter(transform))
		if err != nil {
			ms.logger.CWarnw(ctx, "Skipping a malformed transform from a world state store",
				"store", storeName, "uuid", fmt.Sprintf("%q", uuid), "err", err)
			continue
		}
		if ms.isVisualOnly(ctx, storeName, transform) {
			link = referenceframe.NewLinkInFrame(link.Parent(), link.Pose(), link.Name(), nil)
		}
		ret = append(ret, link)
	}
	return ret
}

// isVisualOnly reports whether a transform's metadata marks it as not to be planned around. Only
// a boolean true does. Anything else leaves the transform an obstacle, which is the safe reading
// of a value that was probably meant as true.
func (ms *builtIn) isVisualOnly(ctx context.Context, storeName string, transform *commonpb.Transform) bool {
	value, ok := transform.GetMetadata().GetFields()[worldstatestore.VisualOnlyMetadataKey]
	if !ok {
		return false
	}
	if _, isBool := value.GetKind().(*structpb.Value_BoolValue); !isBool {
		ms.logger.CWarnw(ctx, "A world state store transform's visual_only is not a boolean. Treating it as an obstacle.",
			"store", storeName, "transform", transform.GetReferenceFrame(), "value", value.AsInterface())
		return false
	}
	return value.GetBoolValue()
}

// placeStoreLinks turns the links of one store into obstacles and supplemental frames. It adds
// the frames it accepts to `knownFrames`.
func (ms *builtIn) placeStoreLinks(
	ctx context.Context, storeName string, links []*referenceframe.LinkInFrame, knownFrames map[string]bool,
) ([]*referenceframe.GeometriesInFrame, []*referenceframe.LinkInFrame) {
	var withGeometry, pending []*referenceframe.LinkInFrame
	for _, link := range links {
		switch {
		case link.Geometry() != nil:
			withGeometry = append(withGeometry, link)
		case knownFrames[link.Name()]:
			// Two frames with one name would make building the frame system fail, and with it the
			// whole move.
			ms.logger.CWarnw(ctx, "Skipping a world state store transform named like an existing frame",
				"store", storeName, "frame", link.Name())
		default:
			pending = append(pending, link)
		}
	}

	// A frame is accepted once its parent is known. Repeating until nothing more is accepted
	// handles frames parented to each other, in whatever order the store listed them.
	var transforms []*referenceframe.LinkInFrame
	for accepted := true; accepted; {
		accepted = false
		for idx, link := range pending {
			if link == nil || !knownFrames[link.Parent()] {
				continue
			}
			pending[idx] = nil
			if knownFrames[link.Name()] {
				ms.logger.CWarnw(ctx, "Skipping a world state store transform named like an existing frame",
					"store", storeName, "frame", link.Name())
				continue
			}
			knownFrames[link.Name()] = true
			transforms = append(transforms, link)
			accepted = true
		}
	}
	for _, link := range pending {
		if link != nil {
			ms.logger.CWarnw(ctx, "Skipping a world state store transform whose parent frame does not exist",
				"store", storeName, "frame", link.Name(), "parent", link.Parent())
		}
	}

	var obstacles []*referenceframe.GeometriesInFrame
	for _, link := range withGeometry {
		if !knownFrames[link.Parent()] {
			ms.logger.CWarnw(ctx, "Skipping a world state store obstacle whose parent frame does not exist",
				"store", storeName, "obstacle", link.Name(), "parent", link.Parent())
			continue
		}

		// The transform's pose is baked into the geometry. The planner later moves it from the
		// parent frame into the world frame.
		geometry := link.Geometry().Transform(link.Pose())
		geometry.SetLabel(link.Name())
		obstacles = append(obstacles, referenceframe.NewGeometriesInFrame(link.Parent(), []spatialmath.Geometry{geometry}))
	}
	return obstacles, transforms
}

// withGeometryCenter gives a transform's geometry an identity center when it has none, which
// converting the geometry requires. The store's message is copied rather than modified.
func withGeometryCenter(transform *commonpb.Transform) *commonpb.Transform {
	geometry := transform.GetPhysicalObject()
	if geometry == nil || geometry.GetCenter() != nil {
		return transform
	}
	ret := proto.Clone(transform).(*commonpb.Transform)
	ret.PhysicalObject.Center = &commonpb.Pose{OZ: 1}
	return ret
}

// frameNames returns the names of the frames in the robot's frame system, including the world
// frame. If the frame system cannot be read only the world frame is known, so what the stores
// hold in any other frame is skipped.
func (ms *builtIn) frameNames(ctx context.Context) map[string]bool {
	ret := map[string]bool{referenceframe.World: true}
	if ms.fsService == nil {
		return ret
	}

	cfg, err := ms.fsService.FrameSystemConfig(ctx)
	if err != nil {
		ms.logger.CWarnw(ctx, "Failed to read the frame system to check world state store frames", "err", err)
		return ret
	}
	for _, part := range cfg.Parts {
		if part == nil || part.FrameConfig == nil {
			continue
		}
		ret[part.FrameConfig.Name()] = true
	}
	return ret
}

// describeObstacles renders each obstacle as "label@parent(x,y,z)" for logging.
func describeObstacles(obstacles []*referenceframe.GeometriesInFrame) string {
	var ret []string
	for _, inFrame := range obstacles {
		for _, geometry := range inFrame.Geometries() {
			center := geometry.Pose().Point()
			ret = append(ret, fmt.Sprintf("%s@%s(%.0f,%.0f,%.0f)", geometry.Label(), inFrame.Parent(), center.X, center.Y, center.Z))
		}
	}
	return strings.Join(ret, ", ")
}

// mergeWorldState combines the request's world state with obstacles and transforms from the
// world state store. Nothing is dropped: when two entries share a name, both are kept and the
// later one is renamed. The request's world state is never modified.
func (ms *builtIn) mergeWorldState(
	ctx context.Context,
	request *referenceframe.WorldState,
	storeObstacles []*referenceframe.GeometriesInFrame,
	storeTransforms []*referenceframe.LinkInFrame,
) *referenceframe.WorldState {
	if len(storeObstacles) == 0 && len(storeTransforms) == 0 {
		return request
	}

	obstacleNames := ms.newUniqueNamer(ctx, "obstacle")
	transformNames := ms.newUniqueNamer(ctx, "transform")
	var obstacles []*referenceframe.GeometriesInFrame
	var transforms []*referenceframe.LinkInFrame
	if request != nil {
		obstacles = ms.appendObstacles(ctx, obstacles, request.Obstacles(), fromRequest, obstacleNames)
		transforms = appendTransforms(transforms, request.Transforms(), fromRequest, transformNames)
	}
	obstacles = ms.appendObstacles(ctx, obstacles, storeObstacles, fromStore, obstacleNames)
	transforms = appendTransforms(transforms, storeTransforms, fromStore, transformNames)

	merged, err := referenceframe.NewWorldState(obstacles, transforms)
	if err != nil {
		// Names were made unique above, so this is not expected.
		ms.logger.CWarnw(ctx, "Failed to merge the world state store into the request's world state. Planning without the store.",
			"err", err)
		return request
	}
	return merged
}

// uniqueNamer returns `name`, or a variation of it if an earlier call already claimed it.
// `source` says where the named thing came from, for the warning logged on a clash.
type uniqueNamer func(name, source string) string

// newUniqueNamer returns a namer for one kind of thing, such as obstacles. Empty names are left
// alone: a world state names its unlabeled obstacles itself.
func (ms *builtIn) newUniqueNamer(ctx context.Context, kind string) uniqueNamer {
	// The source that first claimed each name.
	claimed := make(map[string]string)
	return func(name, source string) string {
		if name == "" {
			return name
		}
		firstSource, exists := claimed[name]
		if !exists {
			claimed[name] = source
			return name
		}

		var ret string
		for suffix := 2; ; suffix++ {
			ret = fmt.Sprintf("%s#%d", name, suffix)
			if _, taken := claimed[ret]; !taken {
				break
			}
		}
		claimed[ret] = source
		ms.logger.CWarnw(ctx, "Two world state entries share a name. Keeping both and renaming the later one.",
			"kind", kind, "name", name, "renamedTo", ret, "first", firstSource, "later", source)
		return ret
	}
}

// appendObstacles appends copies of `src` to `dst` with unique labels. An obstacle group without
// a parent frame is taken to be in the world frame.
func (ms *builtIn) appendObstacles(
	ctx context.Context,
	dst, src []*referenceframe.GeometriesInFrame,
	source string,
	names uniqueNamer,
) []*referenceframe.GeometriesInFrame {
	for _, inFrame := range src {
		if inFrame == nil {
			continue
		}
		parent := inFrame.Parent()
		if parent == "" {
			ms.logger.CWarnw(ctx, "An obstacle group has no parent frame. Assuming the world frame.", "source", source)
			parent = referenceframe.World
		}

		geometries := make([]spatialmath.Geometry, 0, len(inFrame.Geometries()))
		for _, geometry := range inFrame.Geometries() {
			if geometry == nil {
				continue
			}
			// Relabeling modifies a geometry, so work on a copy of the caller's.
			renamed := geometry.Transform(spatialmath.NewZeroPose())
			renamed.SetLabel(names(geometry.Label(), source))
			geometries = append(geometries, renamed)
		}
		if len(geometries) > 0 {
			dst = append(dst, referenceframe.NewGeometriesInFrame(parent, geometries))
		}
	}
	return dst
}

// appendTransforms appends `src` to `dst` with unique names. A transform that has to be renamed
// is rebuilt rather than modified.
func appendTransforms(dst, src []*referenceframe.LinkInFrame, source string, names uniqueNamer) []*referenceframe.LinkInFrame {
	for _, link := range src {
		if link == nil {
			continue
		}
		name := names(link.Name(), source)
		if name != link.Name() {
			link = referenceframe.NewLinkInFrame(link.Parent(), link.Pose(), name, link.Geometry())
		}
		dst = append(dst, link)
	}
	return dst
}
