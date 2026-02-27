package armplanning

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/multierr"
	"go.viam.com/utils/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/referenceframe"
	"go.viam.com/rdk/spatialmath"
)

type tooFarError struct {
	max, want float64
}

func (tfe *tooFarError) Error() string {
	return fmt.Sprintf("asked for a pose too far max: %0.2f, asked for: %0.2f", tfe.max, tfe.want)
}

func (tfe *tooFarError) Is(target error) bool {
	_, ok := target.(*tooFarError)
	return ok
}

var (
	okForSmartCache          = true
	okForSmartCacheBadReason = ""
)

func init() {
	vm, err := mem.VirtualMemory()
	if err != nil {
		panic(err)
	}

	memGB := vm.Total / (1024 * 1024 * 1024)

	if strconv.IntSize < 64 {
		okForSmartCache = false
		okForSmartCacheBadReason = "32-bit system"
	} else if memGB < 2 {
		okForSmartCache = false
		okForSmartCacheBadReason = fmt.Sprintf("not enough ram %v", memGB)
	}
}

// IsTooSmallForCache returns true if we're on a 32-bit system.
func IsTooSmallForCache() bool {
	return !okForSmartCache
}

type smartSeedCacheEntry struct {
	inputs []referenceframe.Input
	pt     r3.Vector
}

type goalCacheBox struct {
	center  r3.Vector
	entries []smartSeedCacheEntry
}

func newCacheForFrame(f referenceframe.Frame, logger logging.Logger) (*cacheForFrame, error) {
	ccf := &cacheForFrame{}

	if IsTooSmallForCache() {
		logger.Warnf("not building cache because " + okForSmartCacheBadReason)
		return ccf, nil
	}

	start := time.Now()

	ccf.entriesForCacheBuilding = make([][]smartSeedCacheEntry, defaultNumThreads)
	perSize := totalCacheSizeEstimate(len(f.DoF())) / defaultNumThreads
	for x := range ccf.entriesForCacheBuilding {
		ccf.entriesForCacheBuilding[x] = make([]smartSeedCacheEntry, 0, perSize+1)
	}

	var mainErr error
	var errLock sync.Mutex

	var wg sync.WaitGroup

	for x := range defaultNumThreads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			values := make([]float64, len(f.DoF()))
			err := ccf.buildCacheHelper(f, values, 0, x)
			if err != nil {
				errLock.Lock()
				mainErr = multierr.Combine(mainErr, err)
				errLock.Unlock()
			}
		}()
	}

	wg.Wait()

	if mainErr != nil {
		return nil, mainErr
	}

	total := 0
	for _, l := range ccf.entriesForCacheBuilding {
		total += len(l)
	}

	logger.Infof("time to do raw building: %v of %d entries (guess %v)", time.Since(start), total, perSize*defaultNumThreads)

	start = time.Now()
	ccf.buildInverseCache()
	logger.Infof("time to buildInverseCache: %v", time.Since(start))

	return ccf, nil
}

type cacheForFrame struct {
	entriesForCacheBuilding [][]smartSeedCacheEntry
	totalSize               int

	maxNorm                    float64
	minCartesian, maxCartesian r3.Vector

	boxes map[string]*goalCacheBox // hash to list
}

func (cff *cacheForFrame) boxKeyCompute(value, min, max float64) int { //nolint: revive
	x := (value - min) / (max - min)
	return int(x * 100)
}

func (cff *cacheForFrame) boxKey(p r3.Vector) string {
	x := cff.boxKeyCompute(p.X, cff.minCartesian.X, cff.maxCartesian.X)
	y := cff.boxKeyCompute(p.Y, cff.minCartesian.Y, cff.maxCartesian.Y)
	z := cff.boxKeyCompute(p.Z, cff.minCartesian.Z, cff.maxCartesian.Z)
	return fmt.Sprintf("%0.3d%0.3d%0.3d", x, y, z)
}

var (
	arm6JogRatios  = []float64{120, 48, 16, 0, 0, 0}
	defaultDivisor = 10.0
)

func totalCacheSizeEstimate(dof int) int {
	if dof != 6 {
		return int(math.Pow(defaultDivisor, float64(dof)))
	}
	l := 1.0
	for _, x := range arm6JogRatios {
		l *= (1 + x)
	}
	return int(l)
}

func (cff *cacheForFrame) buildCacheHelper(f referenceframe.Frame, values []float64, joint, t int) error {
	limits := f.DoF()

	if joint > len(limits) {
		panic(fmt.Errorf("joint: %d > len(limits): %d", joint, len(limits)))
	}

	if joint == len(limits) {
		return cff.addToCache(f, values, t)
	}

	//nolint: revive
	min, max, r := limits[joint].GoodLimits()
	values[joint] = min

	jogDivisor := defaultDivisor
	if len(limits) == 6 {
		// assume it's an arm
		jogDivisor = arm6JogRatios[joint]
	}
	jog := (r / jogDivisor) * .9999
	if jogDivisor == 0 {
		jog = r
		values[joint] = (min + max) / 2
	}
	x := 0
	for values[joint] <= max {
		if joint > 0 || t < 0 || x%defaultNumThreads == t {
			err := cff.buildCacheHelper(f, values, joint+1, t)
			if err != nil {
				return err
			}
		}

		values[joint] += jog
		x++
	}
	return nil
}

func (cff *cacheForFrame) addToCache(frame referenceframe.Frame, inputsNotMine []float64, t int) error {
	inputs := append([]float64{}, inputsNotMine...)
	p, err := frame.Transform(inputs)
	if err != nil {
		return err
	}

	cff.entriesForCacheBuilding[t] = append(cff.entriesForCacheBuilding[t], smartSeedCacheEntry{inputs, p.Point()})

	return nil
}

func (cff *cacheForFrame) buildInverseCache() {
	cff.boxes = map[string]*goalCacheBox{}
	cff.totalSize = 0

	for _, l := range cff.entriesForCacheBuilding {
		for _, e := range l {
			p := e.pt
			cff.minCartesian.X = min(cff.minCartesian.X, p.X)
			cff.minCartesian.Y = min(cff.minCartesian.Y, p.Y)
			cff.minCartesian.Z = min(cff.minCartesian.Z, p.Z)

			cff.maxCartesian.X = max(cff.maxCartesian.X, p.X)
			cff.maxCartesian.Y = max(cff.maxCartesian.Y, p.Y)
			cff.maxCartesian.Z = max(cff.maxCartesian.Z, p.Z)

			cff.totalSize++
		}
	}

	cff.maxNorm = 0.0

	for _, l := range cff.entriesForCacheBuilding {
		for _, e := range l {
			key := cff.boxKey(e.pt)
			box, ok := cff.boxes[key]
			if !ok {
				box = &goalCacheBox{}
				cff.boxes[key] = box
			}
			box.entries = append(box.entries, e)

			box.center = box.center.Add(e.pt)

			cff.maxNorm = max(cff.maxNorm, e.pt.Norm())
		}
	}

	for _, box := range cff.boxes {
		box.center = box.center.Mul(1.0 / float64(len(box.entries)))
	}

	cff.entriesForCacheBuilding = nil
}

func (cff *cacheForFrame) findBoxes(goalPose spatialmath.Pose) []*goalCacheBox {
	type e struct {
		b *goalCacheBox
		d float64
	}

	goalPoint := goalPose.Point()

	best := []e{}
	bestScore := cff.minCartesian.Distance(cff.maxCartesian) / 20

	for _, b := range cff.boxes {
		d := goalPoint.Distance(b.center)
		if d > bestScore*10 {
			continue
		}
		bestScore = min(d, bestScore)
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

func (ssc *smartSeedCache) findMovingInfo(inputs *referenceframe.LinearInputs,
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

	f2w1DQ, err := ssc.fs.GetFrameToWorldTransform(inputs, ssc.fs.Frame(goalPIF.Parent()))
	if err != nil {
		return "", nil, err
	}
	f2w2DQ, err := ssc.fs.GetFrameToWorldTransform(inputs, ssc.fs.Frame(goalFrame))
	if err != nil {
		return "", nil, err
	}
	f2w3DQ, err := ssc.fs.GetFrameToWorldTransform(inputs, ssc.fs.Frame(frame.Name()))
	if err != nil {
		return "", nil, err
	}

	goalInWorld := spatialmath.Compose(goalPIF.Pose(), &spatialmath.DualQuaternion{f2w1DQ})
	delta := spatialmath.PoseDelta(
		&spatialmath.DualQuaternion{f2w3DQ},
		&spatialmath.DualQuaternion{f2w2DQ},
	)

	newPose := spatialmath.Compose(goalInWorld, delta)

	/*
		fmt.Printf("f2w1DQ: %v\n", &spatialmath.DualQuaternion{f2w1DQ})
		fmt.Printf("f2w2DQ: %v\n", &spatialmath.DualQuaternion{f2w2DQ})
		fmt.Printf("f2w3DQ: %v\n", &spatialmath.DualQuaternion{f2w3DQ})
		fmt.Printf("goalFrame: %v\n", goalFrame)
		fmt.Printf("goalInWorld: %v\n", goalInWorld)
		fmt.Printf("delta: %v\n", delta)
		fmt.Printf("eliot: %v -> %v\n", goalPIF, newPose)
	*/

	return frame.Name(), newPose, nil
}

func (ssc *smartSeedCache) findSeed(ctx context.Context,
	goal referenceframe.FrameSystemPoses,
	start *referenceframe.LinearInputs,
	logger logging.Logger,
) (*referenceframe.LinearInputs, error) {
	ss, _, err := ssc.findSeeds(ctx, goal, start, 1, logger)
	if err != nil {
		return nil, err
	}
	if len(ss) == 0 {
		return start, nil
	}
	return ss[0], nil
}

func (ssc *smartSeedCache) findSeeds(ctx context.Context,
	goal referenceframe.FrameSystemPoses,
	start *referenceframe.LinearInputs,
	maxSeeds int,
	logger logging.Logger,
) ([]*referenceframe.LinearInputs, []float64, error) {
	_, span := trace.StartSpan(ctx, "smartSeedCache::findSeeds")
	defer span.End()

	if IsTooSmallForCache() {
		return nil, nil, nil
	}

	if len(goal) > 1 {
		return nil, nil, fmt.Errorf("smartSeedCache findSeed only works with 1 goal for now")
	}

	logger.Debugf("findSeeds goal: %v", goal)

	goalFrame := ""
	var goalPIF *referenceframe.PoseInFrame

	for k, v := range goal {
		goalFrame = k
		goalPIF = v
	}

	movingFrame, movingPose, err := ssc.findMovingInfo(start, goalFrame, goalPIF)
	if err != nil {
		return nil, nil, err
	}

	logger.Debugf("goalPIF: %v movingFrame: %v movingPose: %v", goalPIF, movingFrame, movingPose)
	seeds, divisors, err := ssc.findSeedsForFrame(movingFrame, start.Get(movingFrame), movingPose, maxSeeds, logger)
	if err != nil {
		return nil, nil, err
	}

	fullSeeds := []*referenceframe.LinearInputs{}
	for _, s := range seeds {
		i := referenceframe.NewLinearInputs()
		for k, v := range start.Items() {
			i.Put(k, v)
		}
		i.Put(movingFrame, s)
		fullSeeds = append(fullSeeds, i)
	}

	fullDivisors := start.CopyWithZeros()
	fullDivisors.Put(movingFrame, divisors)

	return fullSeeds, fullDivisors.GetLinearizedInputs(), nil
}

// selectMostVariableEntries selects n entries from the given slice with maximum variability in joint positions
func selectMostVariableEntries(entries []entry, n int) []entry {
	if len(entries) <= n {
		return entries
	}

	selected := make([]entry, 0, n)
	remaining := make([]entry, len(entries))
	copy(remaining, entries)

	// Start with the first entry (best by distance/cost)
	selected = append(selected, remaining[0])
	remaining = remaining[1:]

	// For each subsequent selection, pick the entry that maximizes total variability
	for len(selected) < n && len(remaining) > 0 {
		bestIdx := 0
		bestVariability := -1.0

		for i, candidate := range remaining {
			// Calculate minimum distance to any already selected entry
			minDist := math.MaxFloat64
			for _, sel := range selected {
				dist := myCost(candidate.e.inputs, sel.e.inputs)
				if dist < minDist {
					minDist = dist
				}
			}
			// Select the candidate with the maximum minimum distance (most diverse)
			if minDist > bestVariability {
				bestVariability = minDist
				bestIdx = i
			}
		}

		selected = append(selected, remaining[bestIdx])
		// Remove selected entry from remaining
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

type entry struct {
	e        *smartSeedCacheEntry
	distance float64
	cost     float64
}

func myDistance(start, end r3.Vector) float64 {
	return end.Distance(start)
}

func myCost(start, end []float64) float64 {
	cost := 0.0
	m := 1.0
	for i, s := range start {
		d := math.Abs(end[i] - s)
		cost += (d * m)
		if len(start) == 6 {
			m *= .5
		}
	}
	return cost
}

func (ssc *smartSeedCache) findSeedsForFrame(
	frameName string,
	start []referenceframe.Input,
	goalPose spatialmath.Pose,
	maxSeeds int,
	logger logging.Logger,
) ([][]referenceframe.Input, []float64, error) {
	frame := ssc.fs.Frame(frameName)
	if frame == nil {
		return nil, nil, fmt.Errorf("no frame %s", frameName)
	}

	goalPoint := goalPose.Point()
	n := goalPoint.Norm()
	logger.Debugf("findSeedsForFrame: %s goalPose: %v start: %v norm: %0.2f maxNorm: %0.2f",
		frameName, goalPose, logging.FloatArrayFormat{"", start}, n, ssc.rawCache[frameName].maxNorm)

	if n > ssc.rawCache[frameName].maxNorm {
		return nil, nil, &tooFarError{ssc.rawCache[frameName].maxNorm, n}
	}

	startPose, err := frame.Transform(start)
	if err != nil {
		return nil, nil, err
	}

	startDistance := myDistance(startPose.Point(), goalPoint)

	best := []entry{}

	boxes := ssc.rawCache[frameName].findBoxes(goalPose)

	logger.Debugf("startDistance: %v num boxes: %d", startDistance, len(boxes))

	for _, b := range boxes {
		for _, c := range b.entries {
			distance := myDistance(goalPoint, c.pt)
			if distance > startDistance {
				// we're further than we started, so don't bother
				continue
			}

			cost := myCost(start, c.inputs)
			best = append(best, entry{&c, distance, cost})
		}
	}

	if len(best) == 0 {
		return nil, nil, nil
	}

	// sort by distance then cut

	sort.Slice(best, func(i, j int) bool {
		return best[i].distance < best[j].distance
	})

	cutIdx := 0
	cutDistance := max(1, 2*best[0].distance)
	for cutIdx < len(best) {
		if best[cutIdx].distance > cutDistance {
			break
		}
		cutIdx++
	}

	logger.Debugf("\t len(best): %d cutIdx: %d best distance: %0.2f cutDistance: %0.2f", len(best), cutIdx, best[0].distance, cutDistance)

	best = best[0:cutIdx]

	// sort by cst then cut
	sort.Slice(best, func(i, j int) bool {
		return best[i].cost < best[j].cost
	})

	cutIdx = 0
	costCut := max(3, 5*best[0].cost)
	for cutIdx < len(best) {
		if best[cutIdx].cost > costCut {
			break
		}
		cutIdx++
	}

	logger.Debugf("\t len(best): %d cutIdx: %d best cost: %0.2f costCut: %0.2f", len(best), cutIdx, best[0].cost, costCut)

	best = best[0:cutIdx]

	if maxSeeds <= 0 {
		maxSeeds = len(best)
	}

	if maxSeeds < len(best) {
		// now randomize a bit to get a good set to work with
		best = selectMostVariableEntries(best, maxSeeds)
	}

	var divisors []float64
	if len(frame.DoF()) == 6 {
		for j, r := range arm6JogRatios {
			if j >= 2 {
				divisors = append(divisors, 1)
			} else if r == 0 {
				divisors = append(divisors, 1)
			} else {
				divisors = append(divisors, min(1, 2/(r+1)))
			}
		}
	} else {
		for range len(frame.DoF()) {
			divisors = append(divisors, 1/(1+defaultDivisor))
		}
	}

	ret := [][]referenceframe.Input{}
	for i := 0; i < len(best) && i < maxSeeds; i++ {
		e := best[i]
		logger.Debugf("dist: %02.f cost: %0.2f %v", e.distance, e.cost, logging.FloatArrayFormat{"%0.2f", e.e.inputs})

		similar := false
		for _, other := range ret {
			if similiarInputs(e.e.inputs, other, divisors, frame.DoF()) {
				logger.Debugf("\t skipping %v", logging.FloatArrayFormat{"%0.2f", other})
				similar = true
				break
			}
		}
		if similar {
			continue
		}
		ret = append(ret, e.e.inputs)
	}

	return ret, divisors, nil
}

func similiarInputs(a, b []referenceframe.Input, divisors []float64, limits []referenceframe.Limit) bool {
	for i, l := range limits {
		_, _, r := l.GoodLimits()
		d := math.Abs((a[i] - b[i]) / r)
		if d > divisors[i] {
			return false
		}
	}
	return true
}

var (
	sscCache         = map[int]*cacheForFrame{}
	sscCacheLock     sync.Mutex
	cacheBuildLogger = logging.NewLogger("smart-seed-cache-build")
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
		ccf, err = newCacheForFrame(f, logger)
		if err != nil {
			return err
		}

		cacheBuildLogger.Infof("time to build: %v for: %v size: %d", time.Since(start), frameName, ccf.totalSize)

		sscCacheLock.Lock()
		sscCache[hash] = ccf
		sscCacheLock.Unlock()
	}

	ssc.rawCache[frameName] = ccf

	return nil
}

func (ssc *smartSeedCache) buildCache(logger logging.Logger) error {
	logger.Infof("buildCache # of frames: %d", len(ssc.fs.FrameNames()))

	ssc.rawCache = map[string]*cacheForFrame{}

	for _, frameName := range ssc.fs.FrameNames() {
		err := ssc.buildCacheForFrame(frameName, logger)
		if err != nil {
			return fmt.Errorf("cannot build cache for frame: %s %w", frameName, err)
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

// PrepSmartSeed preps the cache for a FrameSystem.
func PrepSmartSeed(fs *referenceframe.FrameSystem, logger logging.Logger) error {
	_, err := smartSeed(fs, logger)
	return err
}
