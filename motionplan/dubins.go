package motionplan

import (
	"math"
	"sort"
)

// all points are in the form (x, y, psi) except center which are (x, y)

type Dubins struct {
	radius           float64
	point_separation float64
}

type dubinOption struct {
	totalLen   float64
	dubinsPath []float64
	straight   bool
}

func (d *Dubins) find_center(point []float64, side string) []float64 {
	angle := point[2]
	if side == "L" {
		angle += math.Pi / 2
	} else {
		angle -= math.Pi / 2
	}
	center := []float64{point[0] + math.Cos(angle)*d.radius, point[1] + math.Sin(angle)*d.radius}
	return center
}

func (d *Dubins) lsl(start []float64, end []float64, center_0 []float64, center_2 []float64) dubinOption {
	straight_dist := d.dist(center_0, center_2)
	alpha := math.Atan2(d.sub(center_2, center_0)[1], d.sub(center_2, center_0)[0])
	beta_2 := math.Mod((end[2] - alpha), 2*math.Pi)
	beta_0 := math.Mod((alpha - start[2]), 2*math.Pi)
	total_len := d.radius*(beta_2+beta_0) + straight_dist

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = beta_2
	path[2] = straight_dist

	dubin := dubinOption{totalLen: total_len, dubinsPath: path, straight: true}

	return dubin
}

func (d *Dubins) rsr(start []float64, end []float64, center_0 []float64, center_2 []float64) dubinOption {
	straight_dist := d.dist(center_0, center_2)
	alpha := math.Atan2(d.sub(center_2, center_0)[1], d.sub(center_2, center_0)[0])
	beta_2 := math.Mod((-end[2] + alpha), 2*math.Pi)
	beta_0 := math.Mod((-alpha + start[2]), 2*math.Pi)
	total_len := d.radius*(beta_2+beta_0) + straight_dist

	path := make([]float64, 3)
	path[0] = -beta_0
	path[1] = -beta_2
	path[2] = straight_dist

	dubin := dubinOption{totalLen: total_len, dubinsPath: path, straight: true}

	return dubin
}

func (d *Dubins) rsl(start []float64, end []float64, center_0 []float64, center_2 []float64) dubinOption {
	median_point := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(median_point[1], median_point[0])
	half_intercenter := d.norm(median_point)
	if half_intercenter < d.radius {
		zeros := []float64{0., 0., 0.}
		dubin := dubinOption{totalLen: math.Inf(1), dubinsPath: zeros, straight: true}
		return dubin
	}
	alpha := math.Acos(d.radius / half_intercenter)
	beta_0 := math.Mod(-(psia + alpha - start[2] - math.Pi/2), 2*math.Pi)
	beta_2 := math.Mod(math.Pi+end[2]-math.Pi/2-alpha-psia, 2*math.Pi)
	straight_dist := 2 * math.Sqrt((math.Pow(half_intercenter, 2) - math.Pow(d.radius, 2)))
	total_len := d.radius*(beta_2+beta_0) + straight_dist

	path := make([]float64, 3)
	path[0] = -beta_0
	path[1] = beta_2
	path[2] = straight_dist

	dubin := dubinOption{totalLen: total_len, dubinsPath: path, straight: true}

	return dubin
}

func (d *Dubins) lsr(start []float64, end []float64, center_0 []float64, center_2 []float64) dubinOption {
	median_point := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(median_point[1], median_point[0])
	half_intercenter := d.norm(median_point)
	if half_intercenter < d.radius {
		zeros := []float64{0., 0., 0.}
		dubin := dubinOption{totalLen: math.Inf(1), dubinsPath: zeros, straight: true}
		return dubin
	}
	alpha := math.Acos(d.radius / half_intercenter)
	beta_0 := math.Mod((psia - alpha - start[2] + math.Pi/2), 2*math.Pi)
	beta_2 := math.Mod(0.5*math.Pi-end[2]-alpha+psia, 2*math.Pi)
	straight_dist := 2 * math.Sqrt((math.Pow(half_intercenter, 2) - math.Pow(d.radius, 2)))
	total_len := d.radius*(beta_2+beta_0) + straight_dist

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = -beta_2
	path[2] = straight_dist

	dubin := dubinOption{totalLen: total_len, dubinsPath: path, straight: true}

	return dubin
}

func (d *Dubins) lrl(start []float64, end []float64, center_0 []float64, center_2 []float64) dubinOption {
	dist_intercenter := d.dist(center_0, center_2)
	intercenter := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.radius < dist_intercenter && dist_intercenter > 4*d.radius {
		zeros := []float64{0., 0., 0.}
		dubin := dubinOption{totalLen: math.Inf(1), dubinsPath: zeros, straight: true}
		return dubin
	}
	gamma := 2 * math.Asin(dist_intercenter/(4*d.radius))
	beta_0 := math.Mod((psia - start[2] + math.Pi/2 + (math.Pi-gamma)/2), 2*math.Pi)
	beta_1 := math.Mod((-psia + math.Pi/2 + end[2] + (math.Pi-gamma)/2), 2*math.Pi)
	total_len := (2*math.Pi - gamma + math.Abs(beta_0) + math.Abs(beta_1)) * d.radius

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = beta_1
	path[2] = 2*math.Pi - gamma

	dubin := dubinOption{totalLen: total_len, dubinsPath: path, straight: true}

	return dubin
}

func (d *Dubins) rlr(start []float64, end []float64, center_0 []float64, center_2 []float64) dubinOption {
	dist_intercenter := d.dist(center_0, center_2)
	intercenter := []float64{d.sub(center_2, center_0)[0] / 2, d.sub(center_2, center_0)[1] / 2}
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.radius < dist_intercenter && dist_intercenter > 4*d.radius {
		zeros := []float64{0., 0., 0.}
		dubin := dubinOption{totalLen: math.Inf(1), dubinsPath: zeros, straight: true}
		return dubin
	}
	gamma := 2 * math.Asin(dist_intercenter/(4*d.radius))
	beta_0 := math.Mod(-(-psia + (start[2] + math.Pi/2) + (math.Pi-gamma)/2), 2*math.Pi)
	beta_1 := math.Mod(-(psia + math.Pi/2 - end[2] + (math.Pi-gamma)/2), 2*math.Pi)
	total_len := (2*math.Pi - gamma + math.Abs(beta_0) + math.Abs(beta_1)) * d.radius

	path := make([]float64, 3)
	path[0] = beta_0
	path[1] = beta_1
	path[2] = 2*math.Pi - gamma

	dubin := dubinOption{totalLen: total_len, dubinsPath: path, straight: true}

	return dubin
}

func (d *Dubins) all_options(start []float64, end []float64, sorts bool) []dubinOption {
	center_0_left := d.find_center(start, "L")
	center_0_right := d.find_center(start, "R")
	center_2_left := d.find_center(end, "L")
	center_2_right := d.find_center(end, "R")

	options := []dubinOption{d.lsl(start, end, center_0_left, center_2_left),
		d.rsr(start, end, center_0_right, center_2_right),
		d.rsl(start, end, center_0_right, center_2_left),
		d.lsr(start, end, center_0_left, center_2_right),
		d.rlr(start, end, center_0_right, center_2_right),
		d.lrl(start, end, center_0_left, center_2_left)}
	if sorts {
		//sort by first element in options
		sort.SliceStable(options, func(i, j int) bool {
			return options[i].totalLen < options[j].totalLen
		})
	}
	return options
}

func (d *Dubins) generate_points_straight(start []float64, end []float64, path []float64) [][]float64 {
	total := d.radius*(math.Abs(path[1])+math.Abs(path[0])) + path[2]

	center_0 := d.find_center(start, "R")
	center_2 := d.find_center(end, "R")

	if path[0] > 0 {
		center_0 = d.find_center(start, "L")
		center_2 = d.find_center(end, "L")
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
		ini = d.add(center_0, d.mul(sides, d.radius))
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
	// points_len := int(total/d.point_separation)
	var points = make([][]float64, 0)
	x := 0.0
	ind := 0
	for x < total {
		if x < math.Abs(path[0])*d.radius {
			points[ind] = d.circle_arc(start, path[0], center_0, x)
		} else if x > total-math.Abs(path[1])*d.radius {
			points[ind] = d.circle_arc(end, path[1], center_2, x-total)
		} else {
			coeff := (x - math.Abs(path[0])*d.radius) / dist_straight
			points[ind] = d.add(d.mul(fin, coeff), d.mul(ini, (1-coeff)))
		}
		x += d.point_separation
		ind++
	}
	points[ind] = end[:2]
	return points
}

func (d *Dubins) generate_points_curve(start []float64, end []float64, path []float64) [][]float64 {
	total := d.radius*(math.Abs(path[1])+math.Abs(path[0])) + path[2]

	center_0 := d.find_center(start, "R")
	center_2 := d.find_center(end, "R")
	if path[0] > 0 {
		center_0 = d.find_center(start, "L")
		center_2 = d.find_center(end, "L")
	}

	intercenter := d.dist(center_0, center_2)
	center_1 := d.mul(d.add(center_0, center_2), 0.5)
	if path[0] > 0 {
		d.add(center_1, d.mul(d.ortho(d.mul(d.sub(center_2, center_0), 1/intercenter)), math.Sqrt((4*math.Pow(d.radius, 2)-math.Pow((intercenter/2), 2)))))
	} else if path[0] < 0 {
		d.sub(center_1, d.mul(d.ortho(d.mul(d.sub(center_2, center_0), 1/intercenter)), math.Sqrt((4*math.Pow(d.radius, 2)-math.Pow((intercenter/2), 2)))))
	}
	psi_0 := math.Atan2(d.sub(center_1, center_0)[1], d.sub(center_1, center_0)[0]) - math.Pi

	//generate all points
	// points_len := int(total/d.point_separation)
	var points = make([][]float64, 0)
	x := 0.0
	ind := 0
	for x < total {
		if x < math.Abs(path[0])*d.radius {
			points[ind] = d.circle_arc(start, path[0], center_0, x)
		} else if x > total-math.Abs(path[1])*d.radius {
			points[ind] = d.circle_arc(end, path[1], center_2, x-total)
		} else {
			angle := psi_0
			if path[0] > 0 {
				angle += (x/d.radius - math.Abs(path[0]))
			} else if path[0] < 0 {
				angle -= (x/d.radius - math.Abs(path[0]))
			}
			sides := []float64{math.Cos(angle), math.Sin(angle)}
			points[ind] = d.add(center_1, d.mul(sides, d.radius))
		}
		x += d.point_separation
		ind++
	}
	points[ind] = end[:2]
	return points
}

func (d *Dubins) circle_arc(reference []float64, beta float64, center []float64, x float64) []float64 {
	angle := reference[2]
	if beta > 0 {
		angle += ((x / d.radius) - math.Pi/2)
	} else if beta < 0 {
		angle -= ((x / d.radius) - math.Pi/2)
	}
	sides := []float64{math.Cos(angle), math.Sin(angle)}
	point := d.add(center, d.mul(sides, d.radius))
	return point
}

func (d *Dubins) generate_points(start []float64, end []float64, dubins_path []float64, straight bool) [][]float64 {
	if straight {
		return d.generate_points_straight(start, end, dubins_path)
	}
	return d.generate_points_curve(start, end, dubins_path)
}

func (d *Dubins) dubins_path(start []float64, end []float64) [][]float64 {
	options := d.all_options(start, end, true)
	dubins_path, straight := options[0].dubinsPath, options[0].straight
	return d.generate_points(start, end, dubins_path, straight)
}

//Helper functions
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
	var subv []float64
	for i, _ := range vect1 {
		subv[i] = vect1[i] - vect2[i]
	}
	return subv
}

// element wise addition
func (d *Dubins) add(vect1 []float64, vect2 []float64) []float64 {
	var addv []float64
	for i, _ := range vect1 {
		addv[i] = vect1[i] + vect2[i]
	}
	return addv
}

// element wise multiplication by a scalar
func (d *Dubins) mul(vect1 []float64, scalar float64) []float64 {
	var mulv []float64
	for i, x := range vect1 {
		mulv[i] = x * scalar
	}
	return mulv
}
