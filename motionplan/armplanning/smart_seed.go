package armplanning

import (
	"fmt"
	"sort"
	
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

	cache []smartSeedCacheEntry
}

func (ssc *smartSeedCache) findSeed(goal referenceframe.FrameSystemPoses, start referenceframe.FrameSystemInputs) (referenceframe.FrameSystemInputs, error) {
	for _, p := range goal {
		if p.Parent() != "world" {
			return nil, fmt.Errorf("goal has to be in world, not %s", p.Parent())
		}
	}

	type entry struct {
		idx int
		distance float64
		cost float64
	}
	
	best := []entry{}


	bestDistance := 10000000000.0
	
	for idx, c := range ssc.cache {
		distance := 0.0
		for k, p := range goal {
			distance += motionplan.WeightedSquaredNormDistance(p.Pose(), c.poses[k].Pose())
			if distance > (bestDistance * 2) {
				break
			}

		}

		if distance < bestDistance {
			bestDistance = distance
		}

		if distance > (bestDistance * 2) {
			continue
		}
		
		cost := 0.0
		for k, j := range start {
			cost += referenceframe.InputsL2Distance(j, c.inputs[k])
		}
		
		best = append(best, entry{idx, distance, cost})
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
	/*	
	for i := 0; i < len(best) && i < 100; i++ {
		e := best[i]
		fmt.Printf("%v dist: %02.f cost: %0.2f\n", ssc.cache[e.idx].inputs, e.distance, e.cost)
	}
	*/	
	return ssc.cache[best[0].idx].inputs, nil
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
	
	ssc.cache = append(ssc.cache, smartSeedCacheEntry{inputs, poses})
	return nil
}

func (ssc *smartSeedCache) build(values []float64, joint int) error {
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
		err := ssc.build(values, joint + 1)
		if err != nil {
			return err
		}
		values[joint] += jog
	}
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

	values := make([]float64, len(lfs.dof))
	err = c.build(values, 0)
	if err != nil {
		return nil, err
	}

	foooooo[fs] = c
	return c, nil
}
