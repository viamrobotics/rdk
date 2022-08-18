package motionplan

import (
	"errors"
	"math"
	"sort"
	"go.viam.com/rdk/referenceframe"
)

type Dubins struct {
	Radius          float64
	PointSeparation float64
}

type DubinOption struct {
	TotalLen   float64
	DubinsPath []float64
	Straight   bool
}

func NewDubins(radius float64, point_separation float64) (*Dubins, error) {
	if radius <= 0 {
		return nil, errors.New("Radius must be greater than 0")
	}
	if point_separation <= 0 {
		return nil, errors.New("Point Separation must be greater than 0")
	}
	dubins := &Dubins{Radius: radius, PointSeparation: point_separation}
	return dubins, nil
}

func (d *Dubins) findCenter(point []float64, side string) []float64 {
	angle := point[2]
	if side == "L" {
		angle += math.Pi / 2
	} else {
		angle -= math.Pi / 2
	}
	center := []float64{point[0] + math.Cos(angle)*d.Radius, point[1] + math.Sin(angle)*d.Radius}
	return center
}

func (d *Dubins) arc(angle float64) float64 {
	return math.Abs(d.Radius * angle)
}

// Calculates a DubinOption for moving from start to end using a Left, Straight, Left path
func (d *Dubins) lsl(start []float64, end []float64, center_0 []float64, center_2 []float64) DubinOption {
	straight_dist := d.dist(center_0, center_2)
	alpha := math.Atan2(d.sub(center_2, center_0)[1], d.sub(center_2, center_0)[0])
	beta_2 := d.mod((end[2] - alpha), 2*math.Pi)
	beta_0 := d.mod((alpha - start[2]), 2*math.Pi)
	total_len := d.Radius*(beta_2+beta_0) + straight_dist // both

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = beta_2
	path[2] = straight_dist

	dubin := DubinOption{TotalLen: total_len, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinOption for moving from start to end using a Right, Straight, Right path
func (d *Dubins) rsr(start []float64, end []float64, center_0 []float64, center_2 []float64) DubinOption {
	alpha := math.Atan2(d.sub(center_2, center_0)[1], d.sub(center_2, center_0)[0])
	beta_2 := d.mod((-end[2] + alpha), 2*math.Pi)
	beta_0 := d.mod((-alpha + start[2]), 2*math.Pi)
	straight_dist := d.dist(center_0, center_2)
	total_len := d.Radius*(beta_2+beta_0) + straight_dist

	path := make([]float64, 3)
	path[0] = -beta_0
	path[1] = -beta_2
	path[2] = straight_dist

	dubin := DubinOption{TotalLen: total_len, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinOption for moving from start to end using a Right, Straight, Left path
func (d *Dubins) rsl(start []float64, end []float64, center_0 []float64, center_2 []float64) DubinOption {
	median_point := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(median_point[1], median_point[0])
	half_intercenter := d.norm(median_point)
	if half_intercenter < d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinOption{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: true}
		return dubin
	}
	alpha := math.Acos(d.Radius / half_intercenter)
	beta_0 := d.mod(-(psia + alpha - start[2] - math.Pi/2), 2*math.Pi)
	beta_2 := d.mod(math.Pi+end[2]-math.Pi/2-alpha-psia, 2*math.Pi)
	straight_dist := 2 * (math.Sqrt((math.Pow(half_intercenter, 2) - math.Pow(d.Radius, 2))))

	total_len := d.Radius*(beta_2+beta_0) + straight_dist

	path := make([]float64, 3)
	path[0] = -beta_0
	path[1] = beta_2
	path[2] = straight_dist

	dubin := DubinOption{TotalLen: total_len, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinOption for moving from start to end using a Left, Straight, Right path
func (d *Dubins) lsr(start []float64, end []float64, center_0 []float64, center_2 []float64) DubinOption {
	median_point := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(median_point[1], median_point[0])
	half_intercenter := d.norm(median_point)
	if half_intercenter < d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinOption{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: true}
		return dubin
	}
	alpha := math.Acos(d.Radius / half_intercenter)
	beta_0 := d.mod((psia - alpha - start[2] + math.Pi/2), 2*math.Pi)
	beta_2 := d.mod(0.5*math.Pi-end[2]-alpha+psia, 2*math.Pi)
	straight_dist := 2 * (math.Sqrt((math.Pow(half_intercenter, 2) - math.Pow(d.Radius, 2))))
	total_len := d.Radius*(beta_2+beta_0) + straight_dist

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = -beta_2
	path[2] = straight_dist

	dubin := DubinOption{TotalLen: total_len, DubinsPath: path, Straight: true}

	return dubin
}

// Calculates a DubinOption for moving from start to end using a Left, Right, Left path
func (d *Dubins) lrl(start []float64, end []float64, center_0 []float64, center_2 []float64) DubinOption {
	dist_intercenter := d.dist(center_0, center_2)
	intercenter := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.Radius < dist_intercenter && dist_intercenter > 4*d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinOption{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: false}
		return dubin
	}
	gamma := 2 * math.Asin(dist_intercenter/(4*d.Radius))
	beta_0 := d.mod((psia - start[2] + math.Pi/2 + (math.Pi-gamma)/2), 2*math.Pi)
	beta_1 := d.mod((-psia + math.Pi/2 + end[2] + (math.Pi-gamma)/2), 2*math.Pi)
	total_len := (2*math.Pi - gamma + math.Abs(beta_0) + math.Abs(beta_1)) * d.Radius

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = beta_1
	path[2] = 2*math.Pi - gamma

	dubin := DubinOption{TotalLen: total_len, DubinsPath: path, Straight: false}

	return dubin
}

// Calculates a DubinOption for moving from start to end using a Right, Left, Right path
func (d *Dubins) rlr(start []float64, end []float64, center_0 []float64, center_2 []float64) DubinOption {
	dist_intercenter := d.dist(center_0, center_2)
	intercenter := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.Radius < dist_intercenter && dist_intercenter > 4*d.Radius {
		zeros := []float64{0., 0., 0.}
		dubin := DubinOption{TotalLen: math.Inf(1), DubinsPath: zeros, Straight: false}
		return dubin
	}
	gamma := 2 * math.Asin(dist_intercenter/(4*d.Radius))
	beta_0 := -d.mod((-psia + (start[2] + math.Pi/2) + (math.Pi-gamma)/2), 2*math.Pi)
	beta_1 := -d.mod((psia + math.Pi/2 - end[2] + (math.Pi-gamma)/2), 2*math.Pi)
	total_len := (2*math.Pi - gamma + math.Abs(beta_0) + math.Abs(beta_1)) * d.Radius

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = beta_1
	path[2] = 2*math.Pi - gamma

	dubin := DubinOption{TotalLen: total_len, DubinsPath: path, Straight: false}

	return dubin
}

func (d *Dubins) AllOptions(start []float64, end []float64, sorts bool) []DubinOption {
	center_0_left := d.findCenter(start, "L")
	center_0_right := d.findCenter(start, "R")
	center_2_left := d.findCenter(end, "L")
	center_2_right := d.findCenter(end, "R")

	options := []DubinOption{d.lsl(start, end, center_0_left, center_2_left),
		d.rsr(start, end, center_0_right, center_2_right),
		d.rsl(start, end, center_0_right, center_2_left),
		d.lsr(start, end, center_0_left, center_2_right),
		d.rlr(start, end, center_0_right, center_2_right),
		d.lrl(start, end, center_0_left, center_2_left)}
	if sorts {
		//sort by first element in options
		sort.SliceStable(options, func(i, j int) bool {
			return options[i].TotalLen < options[j].TotalLen
		})
	}
	return options
}

func (d *Dubins) generatePointsStraight(start []float64, end []float64, path []float64) [][]float64 {
	total := d.Radius*(math.Abs(path[1])+math.Abs(path[0])) + path[2]

	center_0 := d.findCenter(start, "R")
	center_2 := d.findCenter(end, "R")

	if path[0] > 0 {
		center_0 = d.findCenter(start, "L")
		center_2 = d.findCenter(end, "L")
	}

	//start of straight
	ini := start[:2] //if less than 0
	if math.Abs(path[0]) > 0 {
		angle := start[2]
		if path[0] > 0 {
			angle += (math.Abs(path[0]) - math.Pi/2)
		} else if path[0] < 0 {
			angle -= (math.Abs(path[0]) - math.Pi/2)
		}
		sides := []float64{math.Cos(angle), math.Sin(angle)}
		ini = d.add(center_0, d.mul(sides, d.Radius))
	}

	//end of straight
	fin := end[:2]
	if math.Abs(path[1]) > 0 {
		angle := end[2] + (-math.Abs(path[1]) - math.Pi/2)
		if path[1] > 0 {
			angle += (-math.Abs(path[1]) - math.Pi/2)
		} else if path[1] < 0 {
			angle -= (-math.Abs(path[1]) - math.Pi/2)
		}
		sides := []float64{math.Cos(angle), math.Sin(angle)}
		fin = d.add(center_2, sides)
	}

	dist_straight := d.dist(ini, fin)

	//generate all points
	// points_len := int(total/d.PointSeparation)
	points := make([][]float64, 0)
	x := 0.0
	for x < total {
		if x < math.Abs(path[0])*d.Radius {
			points = append(points, d.circleArc(start, path[0], center_0, x))
		} else if x > total-math.Abs(path[1])*d.Radius {
			points = append(points, d.circleArc(end, path[1], center_2, x-total))
		} else {
			coeff := (x - math.Abs(path[0])*d.Radius) / dist_straight
			points = append(points, d.add(d.mul(fin, coeff), d.mul(ini, (1-coeff))))
		}
		x += d.PointSeparation
	}
	points = append(points, end[:2])
	return points
}

func (d *Dubins) generatePointsCurve(start []float64, end []float64, path []float64) [][]float64 {
	total := d.Radius*(math.Abs(path[1])+math.Abs(path[0])) + path[2]

	center_0 := d.findCenter(start, "R")
	center_2 := d.findCenter(end, "R")
	if path[0] > 0 {
		center_0 = d.findCenter(start, "L")
		center_2 = d.findCenter(end, "L")
	}

	intercenter := d.dist(center_0, center_2)
	center_1 := d.mul(d.add(center_0, center_2), 0.5)
	if path[0] > 0 {
		d.add(center_1, d.mul(d.ortho(d.mul(d.sub(center_2, center_0), 1/intercenter)), math.Sqrt((4*math.Pow(d.Radius, 2)-math.Pow((intercenter/2), 2)))))
	} else if path[0] < 0 {
		d.sub(center_1, d.mul(d.ortho(d.mul(d.sub(center_2, center_0), 1/intercenter)), math.Sqrt((4*math.Pow(d.Radius, 2)-math.Pow((intercenter/2), 2)))))
	}
	psi_0 := math.Atan2(d.sub(center_1, center_0)[1], d.sub(center_1, center_0)[0]) - math.Pi

	//generate all points
	// points_len := int(total/d.PointSeparation)
	points := make([][]float64, 0)
	x := 0.0
	for x < total {
		if x < math.Abs(path[0])*d.Radius {
			points = append(points, d.circleArc(start, path[0], center_0, x))
		} else if x > total-math.Abs(path[1])*d.Radius {
			points = append(points, d.circleArc(end, path[1], center_2, x-total))
		} else {
			angle := psi_0
			if path[0] > 0 {
				angle += (x/d.Radius - math.Abs(path[0]))
			} else if path[0] < 0 {
				angle -= (x/d.Radius - math.Abs(path[0]))
			}
			sides := []float64{math.Cos(angle), math.Sin(angle)}
			points = append(points, d.add(center_1, d.mul(sides, d.Radius)))
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
	point := d.add(center, d.mul(sides, d.Radius))
	return point
}

func (d *Dubins) generatePoints(start []float64, end []float64, DubinsPath []float64, straight bool) [][]float64 {
	if straight {
		return d.generatePointsStraight(start, end, DubinsPath)
	}
	return d.generatePointsCurve(start, end, DubinsPath)
}

func (d *Dubins) DubinsPath(start []float64, end []float64) [][]float64 {
	options := d.AllOptions(start, end, false)
	//sort by first element in options
	sort.SliceStable(options, func(i, j int) bool {
		return options[i].TotalLen < options[j].TotalLen
	})
	DubinsPath, straight := options[0].DubinsPath, options[0].Straight
	return d.generatePoints(start, end, DubinsPath, straight)
}

//Helper functions

// convert to python mod
func (d *Dubins) mod(n float64, M float64) float64 {
	return math.Mod(math.Mod(n, M)+M, M)
}
func (d *Dubins) dist(p1 []float64, p2 []float64) float64 {
	dist := math.Sqrt(math.Pow((p1[0]-p2[0]), 2) + math.Pow((p1[1]-p2[1]), 2))
	return dist
}

func (d *Dubins) norm(p1 []float64) float64 {
	return math.Sqrt(math.Pow(p1[0], 2) + math.Pow(p1[1], 2))
}

func (d *Dubins) ortho(vect []float64) []float64 {
	orth := []float64{-vect[1], vect[0]}
	return orth
}

// element wise subtraction
func (d *Dubins) sub(vect1 []float64, vect2 []float64) []float64 {
	subv := make([]float64, len(vect1))
	for i, _ := range vect1 {
		subv[i] = vect1[i] - vect2[i]
	}
	return subv
}

// element wise addition
func (d *Dubins) add(vect1 []float64, vect2 []float64) []float64 {
	addv := make([]float64, len(vect1))
	for i, _ := range vect1 {
		addv[i] = vect1[i] + vect2[i]
	}
	return addv
}

// element wise multiplication by a scalar
func (d *Dubins) mul(vect1 []float64, scalar float64) []float64 {
	mulv := make([]float64, len(vect1))
	for i, x := range vect1 {
		mulv[i] = x * scalar
	}
	return mulv
}

func GetDubinTrajectoryFromPath(waypoints [][]referenceframe.Input, d Dubins) []DubinOption{
	traj := make([]DubinOption, 0)
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

			pathOptions := d.AllOptions(current, next, true)[0]

			traj = append(traj, pathOptions)

			for j := 0; j < 3; j++ {
				current[j] = next[j]
			}
		}
	}

	return traj
}

