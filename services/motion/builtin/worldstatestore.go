package builtin

import (
	"context"
	"fmt"
	"strings"

	commonpb "go.viam.com/api/common/v1"
	"google.golang.org/protobuf/proto"

	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

// This file feeds the configured world state store into planning. Every `Move` reads the store
// afresh and plans around what it holds, in addition to any world state the request carries.
//
// A problem with the store never fails a `Move`. Whatever cannot be used is logged and skipped,
// because planning with the obstacles that could be read is better than refusing to move.
//
// This is deliberately naive: the store is read with one `GetTransform` call per transform on
// every `Move`, nothing is cached, and only `Move` consults the store. Each of those is a known
// follow-up rather than an oversight.

// The names used in log lines for where a piece of world state came from.
const (
	fromRequest = "request"
	fromStore   = "world state store"
)

// withStoreWorldState returns the request's world state combined with the configured world state
// store's. Without a store, or with an empty one, it returns the request's world state as is.
func (ms *builtIn) withStoreWorldState(ctx context.Context, request *referenceframe.WorldState) *referenceframe.WorldState {
	if ms.worldStateStore == nil {
		return request
	}

	obstacles, transforms := ms.storeWorldState(ctx)
	ms.logger.CDebugw(ctx, "Read the world state store for a move",
		"store", ms.worldStateStore.Name().Name,
		"numObstacles", len(obstacles),
		"numTransforms", len(transforms),
		"obstacles", describeObstacles(obstacles),
	)
	return ms.mergeWorldState(ctx, request, obstacles, transforms)
}

// storeWorldState reads the world state store. A stored transform with a geometry becomes an
// obstacle. One without becomes a supplemental frame that other transforms can be parented to.
func (ms *builtIn) storeWorldState(ctx context.Context) ([]*referenceframe.GeometriesInFrame, []*referenceframe.LinkInFrame) {
	store := ms.worldStateStore
	storeName := store.Name().Name

	uuids, err := store.ListUUIDs(ctx, nil)
	if err != nil {
		ms.logger.CWarnw(ctx, "Failed to list the world state store's transforms. Planning without it.",
			"store", storeName, "err", err)
		return nil, nil
	}

	// The frames an obstacle can be parented to: the robot's, plus the supplemental frames
	// accepted from the store below.
	knownFrames := ms.frameNames(ctx)

	var withGeometry []*referenceframe.LinkInFrame
	var transforms []*referenceframe.LinkInFrame
	for _, uuid := range uuids {
		transform, err := store.GetTransform(ctx, uuid, nil)
		if err != nil {
			ms.logger.CWarnw(ctx, "Failed to get a transform from the world state store. Skipping it.",
				"store", storeName, "uuid", fmt.Sprintf("%q", uuid), "err", err)
			continue
		}
		if transform == nil {
			continue
		}

		link, err := referenceframe.LinkInFrameFromTransformProtobuf(withGeometryCenter(transform))
		if err != nil {
			ms.logger.CWarnw(ctx, "Skipping a malformed transform from the world state store",
				"store", storeName, "uuid", fmt.Sprintf("%q", uuid), "err", err)
			continue
		}

		if link.Geometry() != nil {
			withGeometry = append(withGeometry, link)
			continue
		}

		// A supplemental frame named like an existing frame would make building the frame system
		// fail, and with it the whole move.
		if knownFrames[link.Name()] {
			ms.logger.CWarnw(ctx, "Skipping a world state store transform named like an existing frame",
				"store", storeName, "frame", link.Name())
			continue
		}
		knownFrames[link.Name()] = true
		transforms = append(transforms, link)
	}

	// Obstacles are resolved after the loop so that one can be parented to a supplemental frame
	// regardless of the order the store listed them in.
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
// frame. If the frame system cannot be read only the world frame is known, so store obstacles in
// any other frame are skipped.
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
