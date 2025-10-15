package armplanning

import (
	"fmt"
	"sort"

	"github.com/golang/geo/r3"
	
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/motionplan"
	"go.viam.com/rdk/referenceframe"
)

type smartSeedCacheEntry struct {
	inputs referenceframe.FrameSystemInputs
	poses referenceframe.FrameSystemPoses
}

type smartSeedCache struct {
	fs *referenceframe.FrameSystem
	lfs *linearizedFrameSystem

	rawCache []smartSeedCacheEntry

	//newCache map[string]map[string][]smartSeedCacheEntry
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
	logger.Debugf("startDistance: %v", startDistance)
	
	bestDistance := startDistance * 2
	
	for _, c := range ssc.rawCache {
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


	for i := 0; i < len(best) && i < 5; i++ {
		e := best[i]
		logger.Debugf("%v dist: %02.f cost: %0.2f", e.e.inputs, e.distance, e.cost)
	}

	return best[0].e.inputs, nil
}

func (ssc *smartSeedCache) distance(a, b referenceframe.FrameSystemPoses) float64 {
	dist := 0.0
	
	for k, p := range a {
		if p.Parent() != "world" {
			panic(fmt.Errorf("eliot fucked up %s", p.Parent()))
		}

		pp := b[k]
		
		if pp.Parent() != "world" {
			panic(fmt.Errorf("eliot fucked up %s", pp.Parent()))
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

	for frame, _ := range ssc.rawCache[0].poses {
		err = ssc.buildInverseCache(frame)
		if err != nil {
			return fmt.Errorf("cannot buildInverseCache for %s: %w", frame, err)
		}
	}

	return nil
}

func (ssc *smartSeedCache) buildInverseCache(frame string) error {
	var minCartesian, maxCartesian r3.Vector

	for _, e := range ssc.rawCache {
		minCartesian.X = min(minCartesian.X, e.poses[frame].Pose().Point().X)
		minCartesian.Y = min(minCartesian.X, e.poses[frame].Pose().Point().Y)
		minCartesian.Z = min(minCartesian.X, e.poses[frame].Pose().Point().X)

		maxCartesian.X = max(maxCartesian.X, e.poses[frame].Pose().Point().X)
		maxCartesian.Y = max(maxCartesian.X, e.poses[frame].Pose().Point().Y)
		maxCartesian.Z = max(maxCartesian.X, e.poses[frame].Pose().Point().X)
	}

	fmt.Printf("%v -> %v %v\n", frame, minCartesian, maxCartesian)
	
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


	err = c.buildCache()
	if err != nil {
		return nil, err
	}

	foooooo[fs] = c
	return c, nil
}
