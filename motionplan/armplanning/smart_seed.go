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
	inputs []referenceframe.Input
	pose   spatialmath.Pose // this is in the frame's frame, NOT world
}

type goalCacheBox struct {
	boxKey  string
	center  r3.Vector
	entries []smartSeedCacheEntry
}

func newCacheForFrame(f referenceframe.Frame) (*cacheForFrame, error) {
	ccf := &cacheForFrame{}

	values := make([]float64, len(f.DoF()))

	err := ccf.buildCacheHelper(f, values, 0)
	if err != nil {
		return nil, fmt.Errorf("cannot buildCache for: %s: %w", f.Name(), err)
	}

	ccf.buildInverseCache()

	return ccf, nil
}

type cacheForFrame struct {
	entries []smartSeedCacheEntry

	minCartesian, maxCartesian r3.Vector

	boxes map[string]*goalCacheBox // hash to list
}

func (cff *cacheForFrame) boxKeyCompute(value, min, max float64) int {
	x := (value - min) / (max - min)
	return int(x * 100)
}

func (cff *cacheForFrame) boxKey(p r3.Vector) string {
	x := cff.boxKeyCompute(p.X, cff.minCartesian.X, cff.maxCartesian.X)
	y := cff.boxKeyCompute(p.Y, cff.minCartesian.Y, cff.maxCartesian.Y)
	z := cff.boxKeyCompute(p.Z, cff.minCartesian.Z, cff.maxCartesian.Z)
	return fmt.Sprintf("%0.3d%0.3d%0.3d", x, y, z)
}

func (cff *cacheForFrame) buildCacheHelper(f referenceframe.Frame, values []float64, joint int) error {
	limits := f.DoF()

	if joint > len(limits) {
		panic(fmt.Errorf("joint: %d > len(limits): %d", joint, len(limits)))
	}

	if joint == len(limits) {
		return cff.addToCache(f, values)
	}

	min, max, r := limits[joint].GoodLimits()
	values[joint] = min

	jog := r / 10
	for values[joint] <= max {
		err := cff.buildCacheHelper(f, values, joint+1)
		if err != nil {
			return err
		}

		values[joint] += jog
	}

	return nil
}

func (cff *cacheForFrame) addToCache(frame referenceframe.Frame, inputsNotMine []float64) error {
	inputs := append([]float64{}, inputsNotMine...)
	p, err := frame.Transform(inputs)
	if err != nil {
		return err
	}

	cff.entries = append(cff.entries, smartSeedCacheEntry{inputs, p})

	return nil
}

func (cff *cacheForFrame) buildInverseCache() {
	cff.boxes = map[string]*goalCacheBox{}

	for _, e := range cff.entries {
		cff.minCartesian.X = min(cff.minCartesian.X, e.pose.Point().X)
		cff.minCartesian.Y = min(cff.minCartesian.X, e.pose.Point().Y)
		cff.minCartesian.Z = min(cff.minCartesian.X, e.pose.Point().X)

		cff.maxCartesian.X = max(cff.maxCartesian.X, e.pose.Point().X)
		cff.maxCartesian.Y = max(cff.maxCartesian.X, e.pose.Point().Y)
		cff.maxCartesian.Z = max(cff.maxCartesian.X, e.pose.Point().X)
	}

	for _, e := range cff.entries {
		key := cff.boxKey(e.pose.Point())
		box, ok := cff.boxes[key]
		if !ok {
			box = &goalCacheBox{boxKey: key}
			cff.boxes[key] = box
		}
		box.entries = append(box.entries, e)
	}

	for _, v := range cff.boxes {
		for _, e := range v.entries {
			p := e.pose.Point()
			v.center = v.center.Add(p)
		}

		v.center = v.center.Mul(1.0 / float64(len(v.entries)))
	}
}

func (cff *cacheForFrame) findBoxes(goalPose spatialmath.Pose) []*goalCacheBox {
	type e struct {
		b *goalCacheBox
		d float64
	}

	best := []e{}

	for _, b := range cff.boxes {
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

type smartSeedCache struct {
	fs *referenceframe.FrameSystem

	rawCache map[string]*cacheForFrame
}

func (ssc *smartSeedCache) findMovingInfo(inputs referenceframe.FrameSystemInputs,
	goalFrame string, goalPIF *referenceframe.PoseInFrame,
) (string, spatialmath.Pose, error) {
	var err error
	frame := ssc.fs.Frame(goalFrame)
	if frame == nil {
		return "", nil, fmt.Errorf("no frame for %v", goalFrame)
	}
	for {
		if len(frame.DoF()) > 0 {
			break
		}
		if frame == ssc.fs.World() {
			return "", nil, fmt.Errorf("hit world, and no moving parts when looking to move %s", goalFrame)
		}
		frame, err = ssc.fs.Parent(frame)
		if err != nil {
			return "", nil, err
		}
	}

	// there are 3 frames at play here
	// 1) the frame the goal is specified in
	// 2) the frame of the thing we want to move
	// 3) the frame of the actuating component

	f2w1, err := ssc.fs.GetFrameToWorldTransform(inputs, ssc.fs.Frame(goalPIF.Parent()))
	if err != nil {
		return "", nil, err
	}
	f2w2, err := ssc.fs.GetFrameToWorldTransform(inputs, ssc.fs.Frame(goalFrame))
	if err != nil {
		return "", nil, err
	}
	f2w3, err := ssc.fs.GetFrameToWorldTransform(inputs, ssc.fs.Frame(frame.Name()))
	if err != nil {
		return "", nil, err
	}

	goalInWorld := spatialmath.Compose(goalPIF.Pose(), f2w1)
	delta := spatialmath.Compose(f2w2, spatialmath.PoseInverse(f2w3))

	newPose := spatialmath.Compose(goalInWorld, delta)

	/*
		fmt.Printf("f2w1: %v\n", f2w1)
		fmt.Printf("f2w2: %v\n", f2w2)
		fmt.Printf("f2w3: %v\n", f2w3)
		fmt.Printf("goalInWorld: %v\n", goalInWorld)
		fmt.Printf("delta: %v\n", delta)
		fmt.Printf("eliot: %v -> %v\n", goalPIF, newPose)
	*/

	return frame.Name(), newPose, nil
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

	goalFrame := ""
	var goalPIF *referenceframe.PoseInFrame

	for k, v := range goal {
		goalFrame = k
		goalPIF = v
	}

	movingFrame, movingPose, err := ssc.findMovingInfo(start, goalFrame, goalPIF)
	if err != nil {
		return nil, err
	}

	seeds, err := ssc.findSeedsForFrame(movingFrame, start[movingFrame], movingPose, logger)
	if err != nil {
		return nil, err
	}

	fullSeeds := []referenceframe.FrameSystemInputs{}
	for _, s := range seeds {
		i := referenceframe.FrameSystemInputs{}
		for k, v := range start {
			i[k] = v
		}
		i[goalFrame] = s
		fullSeeds = append(fullSeeds, i)
	}

	return fullSeeds, nil
}

func (ssc *smartSeedCache) findSeedsForFrame(
	frameName string,
	start []referenceframe.Input,
	goalPose spatialmath.Pose,
	logger logging.Logger,
) ([][]referenceframe.Input, error) {
	frame := ssc.fs.Frame(frameName)
	if frame == nil {
		return nil, fmt.Errorf("no frame %s", frameName)
	}

	logger.Debugf("findSeedsForFrame: %s goalPose: %v", frameName, goalPose)

	type entry struct {
		e        *smartSeedCacheEntry
		distance float64
		cost     float64
	}

	startPose, err := frame.Transform(start)
	if err != nil {
		return nil, err
	}

	startDistance := max(1, motionplan.WeightedSquaredNormDistance(startPose, goalPose))

	best := []entry{}

	bestDistance := startDistance * 2

	boxes := ssc.rawCache[frameName].findBoxes(goalPose)

	logger.Debugf("startDistance: %v num boxes: %d", startDistance, len(boxes))

	for _, b := range boxes {
		for _, c := range b.entries {
			distance := motionplan.WeightedSquaredNormDistance(goalPose, c.pose)
			if distance > (bestDistance * 2) {
				continue
			}

			if distance < bestDistance {
				bestDistance = distance
			}

			cost := referenceframe.InputsL2Distance(start, c.inputs)

			best = append(best, entry{&c, distance, cost})
		}
	}

	if len(best) == 0 {
		logger.Debugf("no best, returning start")
		return [][]referenceframe.Input{start}, nil
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

	logger.Debugf("\t len(best): %d cutIdx: %d", len(best), cutIdx)

	best = best[0:cutIdx]

	sort.Slice(best, func(i, j int) bool {
		return best[i].cost < best[j].cost
	})

	ret := [][]referenceframe.Input{}

	for i := 0; i < len(best) && i < 5; i++ {
		e := best[i]
		ret = append(ret, e.e.inputs)
		// logger.Debugf("dist: %02.f cost: %0.2f %v", e.distance, e.cost, e.e.inputs)
	}

	return ret, nil
}

var (
	sscCache     = map[int]*cacheForFrame{}
	sscCacheLock sync.Mutex
)

func (ssc *smartSeedCache) buildCacheForFrame(frameName string, logger logging.Logger) error {
	var err error

	f := ssc.fs.Frame(frameName)
	if f == nil {
		return fmt.Errorf("no frame: %s", f)
	}

	if len(f.DoF()) == 0 {
		return nil
	}

	hash := f.Hash()

	sscCacheLock.Lock()
	ccf, ok := sscCache[hash]
	sscCacheLock.Unlock()

	if !ok {
		start := time.Now()
		ccf, err = newCacheForFrame(f)
		if err != nil {
			return err
		}

		logger.Infof("time to build: %v for: %v", time.Since(start), frameName)

		sscCacheLock.Lock()
		sscCache[hash] = ccf
		sscCacheLock.Unlock()
	}

	ssc.rawCache[frameName] = ccf

	return nil
}

func (ssc *smartSeedCache) buildCache(logger logging.Logger) error {
	logger.Debugf("buildCache # of frames: %d", len(ssc.fs.FrameNames()))

	ssc.rawCache = map[string]*cacheForFrame{}

	for _, frameName := range ssc.fs.FrameNames() {
		err := ssc.buildCacheForFrame(frameName, logger)
		if err != nil {
			return fmt.Errorf("cannot build cache for frame: %s", frameName)
		}
	}

	return nil
}

func smartSeed(fs *referenceframe.FrameSystem, logger logging.Logger) (*smartSeedCache, error) {
	c := &smartSeedCache{
		fs: fs,
	}

	err := c.buildCache(logger)
	if err != nil {
		return nil, err
	}

	return c, nil
}
