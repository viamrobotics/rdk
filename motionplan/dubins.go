package motionplan

import (
	"errors"
	"math"
	"sort"

	"go.viam.com/rdk/referenceframe"
)

// paper: https://arxiv.org/pdf/2206.10533.pdf
// python dubins-rrt package used for reference: https://github.com/FelicienC/RRT-Dubins

// Dubins describes the parameters for a specific Dubin's problem.
type Dubins struct {
	Radius          float64 // Turning radius of car
	PointSeparation float64 // Separation of points on path to check for collision
}

// DubinPathAttr describes a Dubins path that can be taken from one point to another.
type DubinPathAttr struct {
	TotalLen   float64   // Total length of all segments making up a single Dubin's path
	DubinsPath []float64 // Length array of the six possible Dubin's path combiantions
	Straight   bool      // True if Dubin's Path segment is straight
}

// NewDubins creates a new Dubins instance given a valid radius and point separation.
func NewDubins(radius, pointSeparation float64) (*Dubins, error) {
	if radius <= 0 {
		return nil, errors.New("radius must be greater than 0")
	}
	if pointSeparation <= 0 {
		return nil, errors.New("point Separation must be greater than 0")
	}
	dubins := &Dubins{Radius: radius, PointSeparation: pointSeparation}
	return dubins, nil
}

func (d *Dubins) findCenter(point []float64, isLeft bool) []float64 {
	angle := point[2]
	if isLeft {
		angle += math.Pi / 2
	} else {
		angle -= math.Pi / 2
	}
	center := []float64{point[0] + math.Cos(angle)*d.Radius, point[1] + math.Sin(angle)*d.Radius}
	return center
}

// Calculates a DubinPathAttr for moving from start to end using a Left, Straight, Left path.
func (d *Dubins) lsl(start, end, center0, center2 []float64) DubinPathAttr {
	straightDist := dist(center0, center2)
	alpha := math.Atan2(sub(center2, center0)[1], sub(center2, center0)[0])
	beta2 := mod2Pi((end[2] - alpha))
	beta0 := mod2Pi((alpha - start[2]))
	totalLen := d.Radius*(beta2+beta0) + straightDist // both

	path := make([]float64, 3)
	path[0] = beta0
	path[1] = beta2
	path[2] = straightDist

	dubin := DubinPathAttr{TotalLen: totalLen, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinPathAttr for moving from start to end using a Right, Straight, Right path.
func (d *Dubins) rsr(start, end, center0, center2 []float64) DubinPathAttr {
	alpha := math.Atan2(sub(center2, center0)[1], sub(center2, center0)[0])
	beta2 := mod2Pi((-end[2] + alpha))
	beta0 := mod2Pi((-alpha + start[2]))
	straightDist := dist(center0, center2)
	totalLen := d.Radius*(beta2+beta0) + straightDist

	path := make([]float64, 3)
	path[0] = -beta0
	path[1] = -beta2
	path[2] = straightDist

	dubin := DubinPathAttr{TotalLen: totalLen, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinPathAttr for moving from start to end using a Right, Straight, Left path.
func (d *Dubins) rsl(start, end, center0, center2 []float64) DubinPathAttr {
	medianPoint := []float64{sub(center2, center0)[0] / 2, sub(center2, center0)[1] / 2}
	psia := math.Atan2(medianPoint[1], medianPoint[0])
	halfIntercenter := norm(medianPoint)
	if halfIntercenter < d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinPathAttr{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: true}
		return dubin
	}
	alpha := math.Acos(d.Radius / halfIntercenter)
	beta0 := mod2Pi(-(psia + alpha - start[2] - math.Pi/2))
	beta2 := mod2Pi(math.Pi + end[2] - math.Pi/2 - alpha - psia)
	straightDist := 2 * (math.Sqrt((math.Pow(halfIntercenter, 2) - math.Pow(d.Radius, 2))))

	totalLen := d.Radius*(beta2+beta0) + straightDist

	path := make([]float64, 3)
	path[0] = -beta0
	path[1] = beta2
	path[2] = straightDist

	dubin := DubinPathAttr{TotalLen: totalLen, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinPathAttr for moving from start to end using a Left, Straight, Right path.
func (d *Dubins) lsr(start, end, center0, center2 []float64) DubinPathAttr {
	medianPoint := []float64{sub(center2, center0)[0] / 2, sub(center2, center0)[1] / 2}
	psia := math.Atan2(medianPoint[1], medianPoint[0])
	halfIntercenter := norm(medianPoint)
	if halfIntercenter < d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinPathAttr{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: true}
		return dubin
	}
	alpha := math.Acos(d.Radius / halfIntercenter)
	beta0 := mod2Pi((psia - alpha - start[2] + math.Pi/2))
	beta2 := mod2Pi(0.5*math.Pi - end[2] - alpha + psia)
	straightDist := 2 * (math.Sqrt((math.Pow(halfIntercenter, 2) - math.Pow(d.Radius, 2))))
	totalLen := d.Radius*(beta2+beta0) + straightDist

	path := make([]float64, 3)
	path[0] = beta0
	path[1] = -beta2
	path[2] = straightDist

	dubin := DubinPathAttr{TotalLen: totalLen, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinPathAttr for moving from start to end using a Left, Right, Left path.
func (d *Dubins) lrl(start, end, center0, center2 []float64) DubinPathAttr {
	distIntercenter := dist(center0, center2)
	intercenter := []float64{sub(center2, center0)[0] / 2, sub(center2, center0)[1] / 2}
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.Radius < distIntercenter && distIntercenter > 4*d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinPathAttr{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: false}
		return dubin
	}
	gamma := 2 * math.Asin(distIntercenter/(4*d.Radius))
	beta0 := mod2Pi((psia - start[2] + math.Pi/2 + (math.Pi-gamma)/2))
	beta1 := mod2Pi((-psia + math.Pi/2 + end[2] + (math.Pi-gamma)/2))
	totalLen := (2*math.Pi - gamma + math.Abs(beta0) + math.Abs(beta1)) * d.Radius

	path := make([]float64, 3)
	path[0] = beta0
	path[1] = beta1
	path[2] = 2*math.Pi - gamma

	dubin := DubinPathAttr{TotalLen: totalLen, DubinsPath: path, Straight: false}

	return dubin
}

// Calculates a DubinPathAttr for moving from start to end using a Right, Left, Right path.
func (d *Dubins) rlr(start, end, center0, center2 []float64) DubinPathAttr {
	distIntercenter := dist(center0, center2)
	intercenter := []float64{sub(center2, center0)[0] / 2, sub(center2, center0)[1] / 2}
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.Radius < distIntercenter && distIntercenter > 4*d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinPathAttr{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: false}
		return dubin
	}
	gamma := 2 * math.Asin(distIntercenter/(4*d.Radius))
	beta0 := -mod2Pi((-psia + (start[2] + math.Pi/2) + (math.Pi-gamma)/2))
	beta1 := -mod2Pi((psia + math.Pi/2 - end[2] + (math.Pi-gamma)/2))
	totalLen := (2*math.Pi - gamma + math.Abs(beta0) + math.Abs(beta1)) * d.Radius

	path := make([]float64, 3)
	path[0] = beta0
	path[1] = beta1
	path[2] = 2*math.Pi - gamma

	dubin := DubinPathAttr{TotalLen: totalLen, DubinsPath: path, Straight: false}

	return dubin
}

// AllPaths finds every possible Dubins path from start to end and returns them as DubinPathAttrs.
func (d *Dubins) AllPaths(start, end []float64, sorts bool) []DubinPathAttr {
	center0Left := d.findCenter(start, true)   // "L"
	center0Right := d.findCenter(start, false) // "R"
	center2Left := d.findCenter(end, true)     // "L"
	center2Right := d.findCenter(end, false)   // "R"

	paths := []DubinPathAttr{
		d.lsl(start, end, center0Left, center2Left),
		d.rsr(start, end, center0Right, center2Right),
		d.rsl(start, end, center0Right, center2Left),
		d.lsr(start, end, center0Left, center2Right),
		d.rlr(start, end, center0Right, center2Right),
		d.lrl(start, end, center0Left, center2Left),
	}
	if sorts {
		// sort by first element in paths
		sort.SliceStable(paths, func(i, j int) bool {
			return paths[i].TotalLen < paths[j].TotalLen
		})
	}
	return paths
}

func (d *Dubins) generatePointsStraight(start, end, path []float64) [][]float64 {
	total := d.Radius*(math.Abs(path[1])+math.Abs(path[0])) + path[2]

	center0 := d.findCenter(start, false) // "R"
	center2 := d.findCenter(end, false)   // "R"

	if path[0] > 0 {
		center0 = d.findCenter(start, true) // "L"
		center2 = d.findCenter(end, true)   // "L"
	}

	// start of straight
	ini := start[:2]
	if math.Abs(path[0]) > 0 {
		angle := start[2]
		if path[0] > 0 {
			angle += (math.Abs(path[0]) - math.Pi/2)
		} else if path[0] < 0 {
			angle -= (math.Abs(path[0]) - math.Pi/2)
		}
		sides := []float64{math.Cos(angle), math.Sin(angle)}
		ini = add(center0, mul(sides, d.Radius))
	}

	// end of straight
	fin := end[:2]
	if math.Abs(path[1]) > 0 {
		angle := end[2] + (-math.Abs(path[1]) - math.Pi/2)
		if path[1] > 0 {
			angle += (-math.Abs(path[1]) - math.Pi/2)
		} else if path[1] < 0 {
			angle -= (-math.Abs(path[1]) - math.Pi/2)
		}
		sides := []float64{math.Cos(angle), math.Sin(angle)}
		fin = add(center2, sides)
	}

	distStraight := dist(ini, fin)

	// generate all points
	points := make([][]float64, 0)
	x := 0.0
	for x < total {
		switch {
		case x < math.Abs(path[0])*d.Radius:
			points = append(points, d.circleArc(start, path[0], center0, x))
		case x > total-math.Abs(path[1])*d.Radius:
			points = append(points, d.circleArc(end, path[1], center2, x-total))
		default:
			coeff := (x - math.Abs(path[0])*d.Radius) / distStraight
			points = append(points, add(mul(fin, coeff), mul(ini, (1-coeff))))
		}
		x += d.PointSeparation
	}
	points = append(points, end[:2])
	return points
}

func (d *Dubins) generatePointsCurve(start, end, path []float64) [][]float64 {
	total := d.Radius*(math.Abs(path[1])+math.Abs(path[0])) + path[2]

	center0 := d.findCenter(start, false) // "R"
	center2 := d.findCenter(end, false)   // "R"
	if path[0] > 0 {
		center0 = d.findCenter(start, true) // "L"
		center2 = d.findCenter(end, true)   // "L"
	}

	intercenter := dist(center0, center2)
	center1 := mul(add(center0, center2), 0.5)
	if path[0] > 0 {
		add(center1, mul(ortho(mul(sub(center2, center0), 1/intercenter)), math.Sqrt((4*math.Pow(d.Radius, 2)-math.Pow((intercenter/2), 2)))))
	} else if path[0] < 0 {
		sub(center1, mul(ortho(mul(sub(center2, center0), 1/intercenter)), math.Sqrt((4*math.Pow(d.Radius, 2)-math.Pow((intercenter/2), 2)))))
	}
	psi0 := math.Atan2(sub(center1, center0)[1], sub(center1, center0)[0]) - math.Pi

	// generate all points
	points := make([][]float64, 0)
	x := 0.0
	for x < total {
		switch {
		case x < math.Abs(path[0])*d.Radius:
			points = append(points, d.circleArc(start, path[0], center0, x))
		case x > total-math.Abs(path[1])*d.Radius:
			points = append(points, d.circleArc(end, path[1], center2, x-total))
		default:
			angle := psi0
			if path[0] > 0 {
				angle += (x/d.Radius - math.Abs(path[0]))
			} else if path[0] < 0 {
				angle -= (x/d.Radius - math.Abs(path[0]))
			}
			sides := []float64{math.Cos(angle), math.Sin(angle)}
			points = append(points, add(center1, mul(sides, d.Radius)))
		}
		x += d.PointSeparation
	}
	points = append(points, end[:2])
	return points
}

func (d *Dubins) circleArc(reference []float64, beta float64, center []float64, x float64) []float64 {
	angle := reference[2]
	if beta > 0 {
		angle += ((x / d.Radius) - math.Pi/2)
	} else if beta < 0 {
		angle -= ((x / d.Radius) - math.Pi/2)
	}
	sides := []float64{math.Cos(angle), math.Sin(angle)}
	point := add(center, mul(sides, d.Radius))
	return point
}

func (d *Dubins) generatePoints(start, end, dubinsPath []float64, straight bool) [][]float64 {
	if straight {
		return d.generatePointsStraight(start, end, dubinsPath)
	}
	return d.generatePointsCurve(start, end, dubinsPath)
}

// DubinsPath returns a list of points along the shortest Dubins path from start to end.
func (d *Dubins) DubinsPath(start, end []float64) [][]float64 {
	paths := d.AllPaths(start, end, true)
	DubinsPath, straight := paths[0].DubinsPath, paths[0].Straight
	return d.generatePoints(start, end, DubinsPath, straight)
}

// Helper functions

// In python, the modulo computes n%m = (n+m)%n. For example: -1%10 is 9 in python, and -1 in Go/C
// python mod is desirable here so convert to python mod.
func mod2Pi(n float64) float64 {
	const twoPi = 2 * math.Pi
	return math.Mod(math.Mod(n, twoPi)+twoPi, twoPi)
}

func dist(p1, p2 []float64) float64 {
	dist := math.Sqrt(math.Pow((p1[0]-p2[0]), 2) + math.Pow((p1[1]-p2[1]), 2))
	return dist
}

func norm(p1 []float64) float64 {
	return math.Sqrt(math.Pow(p1[0], 2) + math.Pow(p1[1], 2))
}

func ortho(vect []float64) []float64 {
	orth := []float64{-vect[1], vect[0]}
	return orth
}

// element wise subtraction.
func sub(vect1, vect2 []float64) []float64 {
	subv := make([]float64, len(vect1))
	for i := range vect1 {
		subv[i] = vect1[i] - vect2[i]
	}
	return subv
}

// element wise addition.
func add(vect1, vect2 []float64) []float64 {
	addv := make([]float64, len(vect1))
	for i := range vect1 {
		addv[i] = vect1[i] + vect2[i]
	}
	return addv
}

// element wise multiplication by a scalar.
func mul(vect1 []float64, scalar float64) []float64 {
	mulv := make([]float64, len(vect1))
	for i, x := range vect1 {
		mulv[i] = x * scalar
	}
	return mulv
}

// GetDubinTrajectoryFromPath takes a path of waypoints that can be followed using Dubins paths and returns
// a list of DubinPathAttrs describing the Dubins paths to get between waypoints.
func GetDubinTrajectoryFromPath(waypoints [][]referenceframe.Input, d Dubins) []DubinPathAttr {
	traj := make([]DubinPathAttr, 0)
	current := make([]float64, 3)
	next := make([]float64, 3)

	for i, wp := range waypoints {
		if i == 0 {
			for j := 0; j < 3; j++ {
				current[j] = wp[j].Value
			}
		} else {
			for j := 0; j < 3; j++ {
				next[j] = wp[j].Value
			}

			allPaths := d.AllPaths(current, next, true)[0]

			traj = append(traj, allPaths)

			for j := 0; j < 3; j++ {
				current[j] = next[j]
			}
		}
	}

	return traj
}
