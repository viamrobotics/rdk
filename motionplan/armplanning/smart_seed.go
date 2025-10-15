package armplanning

import (
	"fmt"
	"sort"
	"time"

	"github.com/golang/geo/r3"
	
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type smartSeedCacheEntry struct {
	inputs referenceframe.FrameSystemInputs
	poses referenceframe.FrameSystemPoses
}

type goalCacheBox struct {
	hash string
	center r3.Vector
	entries []smartSeedCacheEntry
}

type goalCache struct {
	minCartesian, maxCartesian r3.Vector
	boxes map[string]*goalCacheBox // hash to list
}

func (gc *goalCache) hashKey(value, min, max float64) int {
	x := (value - min) / (max - min)

	d := int(x*10)
	if d >= 10 {
		d = 9
	}
	return d
}

func (gc *goalCache) hash(p r3.Vector) string {
	x := gc.hashKey(p.X, gc.minCartesian.X, gc.maxCartesian.X)
	y := gc.hashKey(p.Y, gc.minCartesian.Y, gc.maxCartesian.Y)
	z := gc.hashKey(p.Z, gc.minCartesian.Z, gc.maxCartesian.Z)
	return fmt.Sprintf("%d%d%d", x, y, z)
}

type smartSeedCache struct {
	fs *referenceframe.FrameSystem
	lfs *linearizedFrameSystem

	rawCache []smartSeedCacheEntry

	geoCache map[string]*goalCache
}

func (scs *smartSeedCache) findBoxes(goalFrame string, goalPose spatialmath.Pose) []*goalCacheBox {
	type e struct {
		b *goalCacheBox
		d float64
	}

	if scs.geoCache[goalFrame] == nil {
		err := scs.buildInverseCache(goalFrame)
		if err != nil {
			panic(err)
		}
	}
	
	best := []e{}

	for _, b := range scs.geoCache[goalFrame].boxes {
		d := goalPose.Point().Distance(b.center)
		best = append(best, e{b,d})
	}

	sort.Slice(best, func(a,b int) bool {
		return best[a].d < best[b].d
	})

	boxes := []*goalCacheBox{}

	for i := 0; i < 10 && i < len(best); i++ {
		boxes = append(boxes, best[i].b)
	}
	
	return boxes
}

func (ssc *smartSeedCache) findSeed(goal referenceframe.FrameSystemPoses, start referenceframe.FrameSystemInputs, logger logging.Logger) (referenceframe.FrameSystemInputs, error) {
	if len(goal) > 1 {
		return nil, fmt.Errorf("smartSeedCache findSeed only works with 1 goal for now")
	}
	
	for _, p := range goal {
		if p.Parent() != "world" {
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
		e *smartSeedCacheEntry 
		distance float64
		cost float64
	}
	
	best := []entry{}


	startPoses, err := start.ComputePoses(ssc.fs)
	if err != nil {
		return nil, err
	}

	startDistance := ssc.distance(startPoses, goal)
	
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
		return start, nil
	}
	
	sort.Slice(best, func(i,j int) bool {
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

	sort.Slice(best, func(i,j int) bool {
		return best[i].cost < best[j].cost
	})

	if false {
		for i := 0; i < len(best) && i < 5; i++ {
			e := best[i]
			logger.Debugf("%v dist: %02.f cost: %0.2f", e.e.inputs, e.distance, e.cost)
		}
	}
	
	return best[0].e.inputs, nil
}

func (ssc *smartSeedCache) distance(a, b referenceframe.FrameSystemPoses) float64 {
	dist := 0.0
	
	for k, p := range a {
		if p.Parent() != "world" {
			panic(fmt.Errorf("eliot fucked up %s", p.Parent()))
		}

		pp, ok := b[k]
		if !ok {
			continue
		}
		
		if pp == nil || pp.Parent() != "world" {
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
		if p.Parent() != "world" {
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
		err := ssc.buildRawCache(values, joint + 1)
		if err != nil {
			return err
		}
		values[joint] += jog
	}
	return nil
}

func (ssc *smartSeedCache) buildCache() error {
	values := make([]float64, len(ssc.lfs.dof))
	err := ssc.buildRawCache(values, 0)
	if err != nil {
		return fmt.Errorf("cannot buildCache: %w", err)
	}

	ssc.geoCache = map[string]*goalCache{}
	/*
	for frame, _ := range ssc.rawCache[0].poses {
		err = ssc.buildInverseCache(frame)
		if err != nil {
			return fmt.Errorf("cannot buildInverseCache for %s: %w", frame, err)
		}
	}
	*/
	return nil
}

func (ssc *smartSeedCache) buildInverseCache(frame string) error {
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
		hash := gc.hash(e.poses[frame].Pose().Point())
		box, ok := gc.boxes[hash]
		if !ok {
			box = &goalCacheBox{hash: hash}
			gc.boxes[hash] = box
		}
		box.entries = append(box.entries, e)
	}

	for _, v := range gc.boxes {
		for _, e := range v.entries {
			p := e.poses[frame].Pose().Point()
			v.center = v.center.Add(p)
		}

		v.center = v.center.Mul( 1.0 / float64(len(v.entries)) )
	}
	
	ssc.geoCache[frame] = gc
	
	return nil

}

var foooooo map[*referenceframe.FrameSystem]*smartSeedCache

func smartSeed(fs *referenceframe.FrameSystem) (*smartSeedCache, error) {
	if foooooo == nil {
		foooooo = map[*referenceframe.FrameSystem]*smartSeedCache{}
	}
	c, ok := foooooo[fs]
	fmt.Printf("ok: %v\n", ok)
	if ok {
		return c, nil
	}
	
	lfs, err := newLinearizedFrameSystem(fs)
	if err != nil {
		return nil, err
	}

	c = &smartSeedCache{
		fs: fs,
		lfs: lfs,
	}

	start := time.Now()
	err = c.buildCache()
	if err != nil {
		return nil, err
	}
	fmt.Printf("time to build: %v dof: %v\n", time.Since(start), len(lfs.dof))
	foooooo[fs] = c
	return c, nil
}
