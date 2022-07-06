package motionplan

import (
	"errors"
	"fmt"
	"math"
)

// all points are in the form (x, y, psi) except center which are (x, y)

type Dubins struct {
	radius					float64
	point_separation		float64
}

func (d *Dubins) find_center(point []float64, side string) []float64 {
	angle := point[2]
	if side=="L" {
		angle += math.Pi/2
	} else {
		angle -= math.Pi/2
	}
	center := make([]float64, 2)
	center.append(point[0]+math.Cos(angle)*d.radius)
	center.append(point[1]+math.Sin(angle)*d.radius)
	return center
}

func (d *Dubins) lsl(start []float64, end []float64, center_0 []float64, center_2 []float64) (float64, []float64, bool) {
	straight_dist := d.dist(center_0, center_2)
	alpha := math.Atan2((center_2-center_1)[1], (center_2-center_0)[0])
	beta_2 := math.Mod((end[2]-alpha), 2*math.Pi)
	beta_0 := math.Mod((alpha-start[2]), 2*math.Pi)
	total_len := d.radius*(beta_2+beta_0)+straight_dist

	path := make([]float64, 3)
	path.append(beta_0)
	path.append(beta_2)
	path.append(straight_dist)

	return tota_len, path, true
}

func (d *Dubins) rsr(start []float64, end []float64, center_0 []float64, center_2 []float64) (float64, []float64, bool) {
	straight_dist := d.dist(center_0, center_2)
	alpha := math.Atan2((center_2-center_1)[1], (center_2-center_0)[0])
	beta_2 := math.Mod((-end[2]+alpha), 2*math.Pi)
	beta_0 := math.Mod((-alpha+start[2]), 2*math.Pi)
	total_len := d.radius*(beta_2+beta_0)+straight_dist

	path := make([]float64, 3)
	path.append(-beta_0)
	path.append(-beta_2)
	path.append(straight_dist)

	return tota_len, path, true
}

func (d *Dubins) rsl(start []float64, end []float64, center_0 []float64, center_2 []float64) (float64, []float64, bool) {
	median_point := (center_2 - center_0)/2
	psia := math.Atan2(median_point[1], median_point[0])
	half_intercenter = d.norm(median_point)
	if half_intercenter < d.radius {
		var zeros [3]float64
		return math.Inf(1), zeros, true
	}
	alpha = math.Acos(d.radius/half_intercenter)
	beta_0 = math.Mod(-(psia+alpha-start[2] - math.Pi/2), 2*math.Pi)
	beta_2 = math.Mod(math.Pi+end[2]-math.Pi/2-alpha-psia, 2*math.Pi)
	straight_dist = 2*math.Sqrt((math.Pow(half_intercenter, 2)-math.Pow(d.radius, 2)))
	total_len = d.radius*(beta_2+beta_0)+straight_dist

	path := make([]float64, 3)
	path.append(-beta_0)
	path.append(beta_2)
	path.append(straight_dist)

	return total_len, path, true
}

func (d *Dubins) lsr(start []float64, end []float64, center_0 []float64, center_2 []float64) (float64, []float64, bool) {
	median_point := (center_2 - center_0)/2
	psia := math.Atan2(median_point[1], median_point[0])
	half_intercenter = d.norm(median_point)
	if half_intercenter < d.radius {
		var zeros [3]float64
		return math.Inf(1), zeros, true
	}
	alpha = math.Acos(d.radius/half_intercenter)
	beta_0 = math.Mod((psia-alpha-start[2] + math.Pi/2), 2*math.Pi)
	beta_2 = math.Mod(0.5*math.Pi-end[2]-alpha+psia, 2*math.Pi)
	straight_dist = 2*math.Sqrt((math.Pow(half_intercenter, 2)-math.Pow(d.radius, 2)))
	total_len = d.radius*(beta_2+beta_0)+straight_dist

	path := make([]float64, 3)
	path.append(beta_0)
	path.append(-beta_2)
	path.append(straight_dist)

	return total_len, path, true
}

func (d *Dubins) lrl(start []float64, end []float64, center_0 []float64, center_2 []float64) (float64, []float64, bool) {
	dist_intercenter := d.dist(center_0, center_2)
	intercenter := (center_2-center_0)/2
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.radius < dist_intercenter && dist_intercenter > 4*d.radius {
		var zeros [3]float64
		return math.Inf(1), zeros, true
	}
	gamma := 2*math.Asin(dist_intercenter/(4*d.radius))
	beta_0 := math.Mod((psia-start[2]+math.Pi/2+(math.Pi-gamma)/2), 2*math.Pi)
	beta_1 := math.Mod((-psia+math.Pi/2+end[2]+(math.Pi-gamma)/2), 2*math.Pi)
	total_len := (2*math.Pi-gamma+math.Abs(beta_0)+math.Abs(beta_1))*d.radius

	path := make([]float64, 3)
	path.append(beta_0)
	path.append(beta_2)
	path.append(2*math.Pi-gamma)

	return total_len, path, false
}

func (d *Dubins) rlr(start []float64, end []float64, center_0 []float64, center_2 []float64) (float64, []float64, bool) {
	dist_intercenter := d.dist(center_0, center_2)
	intercenter := (center_2-center_0)/2
	psia := math.Atan2(intercenter[1], intercenter[0])
	if 2*d.radius < dist_intercenter && dist_intercenter > 4*d.radius {
		var zeros [3]float64
		return math.Inf(1), zeros, true
	}
	gamma := 2*math.Asin(dist_intercenter/(4*d.radius))
	beta_0 := math.Mod(-(-psia+(start[2]+math.Pi/2)+(math.Pi-gamma)/2), 2*math.Pi)
	beta_1 := math.Mod(-(psia+math.Pi/2-end[2]+(math.Pi-gamma)/2), 2*math.Pi)
	total_len := (2*math.Pi-gamma+math.Abs(beta_0)+math.Abs(beta_1))*d.radius

	path := make([]float64, 3)
	path.append(beta_0)
	path.append(beta_2)
	path.append(2*math.Pi-gamma)

	return total_len, path, false
}


func (d *Dubins) all_options(start []float64, end []float64, sort bool) ([][]float64){
	center_0_left := d.find_center(start, 'L')
	center_0_right := d.find_center(start, 'R')
	center_2_left := d.find_center(end, 'L')
	center_2_right := d.find_center(end, 'R')
	options := [][]float64 {d.lsl(start, end, center_0_left, center_2_left),
				d.rsr(start, end, center_0_right, center_2_right),
				d.rsl(start, end, center_0_right, center_2_left),
				d.lsr(start, end, center_0_left, center_2_right),
				d.rlr(start, end, center_0_right, center_2_right),
				d.lrl(start, end, center_0_left, center_2_left)}
	if sort{
		fmt.Println("no sorting yet")
	}
	return options
}

func (d *Dubins) generate_points_straight(start []float64, end []float64, path []float64) []float64 {
	total := d.radius*(math.Abs(path[1])+math.Abs(path[0]))+path[2]
	if path[0] > 0 {
		center_0 := d.find_center(start, 'L')
		center_2 := d.find_center(end, 'L')
	} else {
		center_0 := d.find_center(start, 'R')
		center_2 := d.find_center(end, 'R')
	}

	//start of straight
	if math.Abs(path[0]) > 0 {
		angle := start[2] + (math.Abs(path[0])-math.Pi/2)*math.Signbit(path[0])
		sides := []float64 {math.Cos(angle), math.Sin(angle)}
		ini := center_0+d.radius*sides
	} else {
		ini := start[:2]
	}
	//end of straight
	if math.Abs(path[1]) > 0 {
		angle = end[2] + (-math.Abs(path[1])-math.Pi/2)*math.Signbit(path[1])
		sides = []float64 {math.Cos(angle), math.Sin(angle)}
		fin := center_2 + sides
	} else {
		fin := end[:2]
	}

	dist_straight = d.dist(ini, fin)

	//generate all points
	points_len := int(total/d.point_separation)
	points := make([][]float64, points_len)
	x := 0.0 
	for x < total {
		if x < math.Abs(path[0])*d.radius {
			points.append(d.circle_arc(start, path[0], center_0, x))
		} else if x > total-math.Abs(path[1])*d.radius {
			points.append(d.circle_arc(end, path[1], center_2, x-total))
		} else {
			coeff := (x-math.Abs(path[0])*d.radius)/dist_straight
			points.append(coeff*fin + (1-coeff)*ini)
		}
		x += d.point_separation
	}
	points.append(end[:2])
	return points
}

func (d *Dubins) generate_points_curve(start []float64, end []float64, path []float64) []float64 {
	total := d.radius*(math.Abs(path[1])+math.Abs(path[0]))+path[2]
	if path[0] > 0 {
		center_0 := d.find_center(start, 'L')
		center_2 := d.find_center(end, 'L')
	} else {
		center_0 := d.find_center(start, 'R')
		center_2 := d.find_center(end, 'R')
	}
	intercenter := d.dist(center_0, center_2)
	center_1 := (center_0+center_2)/2 + math.Signbit(path[0])
				*d.ortho((center_2-center_0)/intercenter)*math.Sqrt((4*math.Pow(d.radius, 2)-math.Pow((intercenter/2), 2)))   
	psi_0 := math.Atan2((center_1-center_0)[1], (center_1-center_0)[0])-math.Pi

	//generate all points
	points_len := int(total/d.point_separation)
	points := make([][]float64, points_len)
	x := 0.0 
	for x < total {
		if x < math.Abs(path[0])*d.radius {
			points.append(d.circle_arc(start, path[0], center_0, x))
		} else if x > total-math.Abs(path[1])*d.radius {
			points.append(d.circle_arc(end, path[1], center_2, x-total))
		} else {
			angle := psi_0-math.Signbit(path[0])*(x/d.radius-math.Abs(path[0]))
			sides := []float64 {math.Cos(angle), math.Sin(angle)}
			points.append(center_1+d.radius*sides)
		}
		x += d.point_separation
	}
	points.append(end[:2])
	return points
} 

func (d *Dubins) circle_arc(reference float64, beta float64, center []float64, x float) {
	angle := reference[2]+((x/d.radius)-math.Pi/2)*math.Signbit(beta)
	sides := []float64 {math.Cos(angle), math.Sin(angle)}
	point := center+d.radius*sides
	return point
}

func (d *Dubins) generate_points(start []float64, end []float64, dubins_path []float64, straight bool) [][]float64 {
	if straight {
		return d.generate_points_straight(start, end, dubins_path)
	}
	return d.generate_points_curve(start, end, dubins_path)
}

func (d *Dubins) dubins_path(start []float64, end []float64) ([][]float64) {
	options := d.all_options(start, end, false)
	//sort by first element in options
	sort.SliceStable(options, func(i, j int) bool {
		return options[i][0] < options[j][0]
	})
	dubins_path, straight := options[1:]
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
	orth := []float64 {-vect[1], vect[0]}
	return orth
}
