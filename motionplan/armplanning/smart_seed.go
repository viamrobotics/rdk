package armplanning

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/golang/geo/r3"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type smartSeedCacheEntry struct {
	inputs referenceframe.FrameSystemInputs
	poses  referenceframe.FrameSystemPoses
}

type goalCacheBox struct {
	boxKey  string
	center  r3.Vector
	entries []smartSeedCacheEntry
}

type goalCache struct {
	minCartesian, maxCartesian r3.Vector
	boxes                      map[string]*goalCacheBox // hash to list
}

func (gc *goalCache) boxKeyCompute(value, min, max float64) int {
	x := (value - min) / (max - min)

	return int(x * 100)
}

func (gc *goalCache) boxKey(p r3.Vector) string {
	x := gc.boxKeyCompute(p.X, gc.minCartesian.X, gc.maxCartesian.X)
	y := gc.boxKeyCompute(p.Y, gc.minCartesian.Y, gc.maxCartesian.Y)
	z := gc.boxKeyCompute(p.Z, gc.minCartesian.Z, gc.maxCartesian.Z)
	return fmt.Sprintf("%0.3d%0.3d%0.3d", x, y, z)
}

type smartSeedCache struct {
	fs  *referenceframe.FrameSystem
	lfs *linearizedFrameSystem

	rawCache []smartSeedCacheEntry

	geoCache map[string]*goalCache
}

func (ssc *smartSeedCache) findBoxes(goalFrame string, goalPose spatialmath.Pose) []*goalCacheBox {
	type e struct {
		b *goalCacheBox
		d float64
	}

	if ssc.geoCache[goalFrame] == nil {
		ssc.buildInverseCache(goalFrame)
	}

	best := []e{}

	for _, b := range ssc.geoCache[goalFrame].boxes {
		d := goalPose.Point().Distance(b.center)
		best = append(best, e{b, d})
	}

	sort.Slice(best, func(a, b int) bool {
		return best[a].d < best[b].d
	})

	boxes := []*goalCacheBox{}

	for i := 0; i < 100 && i < len(best); i++ {
		boxes = append(boxes, best[i].b)
	}

	return boxes
}

func (ssc *smartSeedCache) findSeed(goal referenceframe.FrameSystemPoses,
	start referenceframe.FrameSystemInputs,
	logger logging.Logger,
) (referenceframe.FrameSystemInputs, error) {
	ss, err := ssc.findSeeds(goal, start, logger)
	if err != nil {
		return nil, err
	}
	if len(ss) == 0 {
		return nil, fmt.Errorf("no findSeeds results")
	}
	return ss[0], nil
}

func (ssc *smartSeedCache) findSeeds(goal referenceframe.FrameSystemPoses,
	start referenceframe.FrameSystemInputs,
	logger logging.Logger,
) ([]referenceframe.FrameSystemInputs, error) {
	if len(goal) > 1 {
		return nil, fmt.Errorf("smartSeedCache findSeed only works with 1 goal for now")
	}

	for _, p := range goal {
		if p.Parent() != referenceframe.World {
			return nil, fmt.Errorf("goal has to be in world, not %s", p.Parent())
		}
	}

	goalFrame := ""
	var goalPose spatialmath.Pose

	for k, v := range goal {
		goalFrame = k
		goalPose = v.Pose()
	}

	type entry struct {
		e        *smartSeedCacheEntry
		distance float64
		cost     float64
	}

	best := []entry{}

	startPoses, err := start.ComputePoses(ssc.fs)
	if err != nil {
		return nil, err
	}

	startDistance := max(1, ssc.distance(startPoses, goal))
	logger.Debugf("startDistance: %v", startDistance)
	bestDistance := startDistance * 2

	boxes := ssc.findBoxes(goalFrame, goalPose)

	for _, b := range boxes {
		for _, c := range b.entries {
			distance := ssc.distance(goal, c.poses)
			if distance > (bestDistance * 2) {
				continue
			}

			if distance < bestDistance {
				bestDistance = distance
			}

			cost := 0.0
			for k, j := range start {
				cost += referenceframe.InputsL2Distance(j, c.inputs[k])
			}

			best = append(best, entry{&c, distance, cost})
		}
	}

	if len(best) == 0 {
		logger.Debugf("no best, returning start")
		return []referenceframe.FrameSystemInputs{start}, nil
	}

	sort.Slice(best, func(i, j int) bool {
		return best[i].distance < best[j].distance
	})

	cutIdx := 0
	for cutIdx < len(best) {
		if best[cutIdx].distance > (2 * best[0].distance) {
			break
		}
		cutIdx++
	}

	best = best[0:cutIdx]

	sort.Slice(best, func(i, j int) bool {
		return best[i].cost < best[j].cost
	})

	ret := []referenceframe.FrameSystemInputs{}

	for i := 0; i < len(best) && i < 5; i++ {
		e := best[i]
		ret = append(ret, e.e.inputs)
		// logger.Debugf("%v dist: %02.f cost: %0.2f", e.e.inputs, e.distance, e.cost)
	}

	return ret, nil
}

func (ssc *smartSeedCache) distance(a, b referenceframe.FrameSystemPoses) float64 {
	dist := 0.0

	for k, p := range a {
		if p.Parent() != referenceframe.World {
			panic(fmt.Errorf("eliot fucked up %s", p.Parent()))
		}

		pp, ok := b[k]
		if !ok {
			continue
		}

		if pp == nil || pp.Parent() != referenceframe.World {
			panic(fmt.Errorf("eliot fucked up %s", pp))
		}

		dist += motionplan.WeightedSquaredNormDistance(p.Pose(), pp.Pose())
	}

	return dist
}

func (ssc *smartSeedCache) addToCache(values []float64) error {
	inputs, err := ssc.lfs.sliceToMap(values)
	if err != nil {
		return err
	}
	poses, err := inputs.ComputePoses(ssc.fs)
	if err != nil {
		return err
	}

	for _, p := range poses {
		if p.Parent() != referenceframe.World {
			return fmt.Errorf("why not in world, but %s", p.Parent())
		}
	}

	ssc.rawCache = append(ssc.rawCache, smartSeedCacheEntry{inputs, poses})
	return nil
}

func (ssc *smartSeedCache) buildRawCache(values []float64, joint int) error {
	if joint > len(ssc.lfs.dof) {
		panic(fmt.Errorf("joint: %d > len(ssc.lfs.dof): %d", joint, len(ssc.lfs.dof)))
	}

	if joint == len(ssc.lfs.dof) {
		return ssc.addToCache(values)
	}

	min, max, r := ssc.lfs.dof[joint].GoodLimits()
	values[joint] = min

	jog := r / 10
	for values[joint] <= max {
		err := ssc.buildRawCache(values, joint+1)
		if err != nil {
			return err
		}

		values[joint] += jog
	}

	return nil
}

func (ssc *smartSeedCache) buildCache(logger logging.Logger) error {
	logger.Debugf("buildCache %v", ssc.lfs.dof)

	values := make([]float64, len(ssc.lfs.dof))
	err := ssc.buildRawCache(values, 0)
	if err != nil {
		return fmt.Errorf("cannot buildCache: %w", err)
	}

	ssc.geoCache = map[string]*goalCache{}

	return nil
}

func (ssc *smartSeedCache) buildInverseCache(frame string) {
	gc := &goalCache{
		boxes: map[string]*goalCacheBox{},
	}

	for _, e := range ssc.rawCache {
		gc.minCartesian.X = min(gc.minCartesian.X, e.poses[frame].Pose().Point().X)
		gc.minCartesian.Y = min(gc.minCartesian.X, e.poses[frame].Pose().Point().Y)
		gc.minCartesian.Z = min(gc.minCartesian.X, e.poses[frame].Pose().Point().X)

		gc.maxCartesian.X = max(gc.maxCartesian.X, e.poses[frame].Pose().Point().X)
		gc.maxCartesian.Y = max(gc.maxCartesian.X, e.poses[frame].Pose().Point().Y)
		gc.maxCartesian.Z = max(gc.maxCartesian.X, e.poses[frame].Pose().Point().X)
	}

	for _, e := range ssc.rawCache {
		key := gc.boxKey(e.poses[frame].Pose().Point())
		box, ok := gc.boxes[key]
		if !ok {
			box = &goalCacheBox{boxKey: key}
			gc.boxes[key] = box
		}
		box.entries = append(box.entries, e)
	}

	for _, v := range gc.boxes {
		for _, e := range v.entries {
			p := e.poses[frame].Pose().Point()
			v.center = v.center.Add(p)
		}

		v.center = v.center.Mul(1.0 / float64(len(v.entries)))
	}

	ssc.geoCache[frame] = gc
}

var (
	sscCache     map[int]*smartSeedCache = map[int]*smartSeedCache{}
	sscCacheLock sync.Mutex
)

func smartSeed(fs *referenceframe.FrameSystem, logger logging.Logger) (*smartSeedCache, error) {
	hash := fs.Hash()
	var c *smartSeedCache

	sscCacheLock.Lock()
	c, ok := sscCache[hash]
	sscCacheLock.Unlock()

	if ok {
		return c, nil
	}

	lfs, err := newLinearizedFrameSystem(fs)
	if err != nil {
		return nil, err
	}

	c = &smartSeedCache{
		fs:  fs,
		lfs: lfs,
	}

	start := time.Now()
	err = c.buildCache(logger)
	if err != nil {
		return nil, err
	}
	logger.Warnf("time to build: %v dof: %v rawCache size: %d hash: %v", time.Since(start), len(lfs.dof), len(c.rawCache), hash)

	sscCacheLock.Lock()
	sscCache[hash] = c
	sscCacheLock.Unlock()

	return c, nil
}
