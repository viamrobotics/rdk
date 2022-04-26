package calibrate

import (
	"errors"
	"image"
	"image/draw"
	"math"
	"math/rand"
	"os"
	"strconv"

	"github.com/montanaflynn/stats"
	"go.viam.com/rdk/rimage"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

type Corner struct {
	X float64
	Y float64
	R float64 //Cornerness
}

//NewCorner creates a new corner without a value for R
func NewCorner(x, y float64) Corner {
	return Corner{X: x, Y: y}
}

//NewCornerWithR creates a new corner with an R value
func NewCornerWithR(x, y float64, r float64) Corner {
	return Corner{X: x, Y: y, R: r}
}

//AreEqual is a simple equality test for corners. If all fields are equal, same corner.
func AreEqual(a, b Corner) bool {
	if a.X == b.X && a.Y == b.Y && a.R == b.R {
		return true
	}
	return false
}

//NormalizeCorners takes in a list of corners and returns a list of corners such that the output list
//has x and y values that are (x - xmean)/std(x)
func NormalizeCorners(corners []Corner) []Corner {
	var xSlice, ySlice []float64
	for _, c := range corners {
		xSlice = append(xSlice, float64(c.X))
		ySlice = append(ySlice, float64(c.Y))
	}

	xMean, _ := stats.Mean(xSlice)
	yMean, _ := stats.Mean(ySlice)
	sd_x, _ := stats.StandardDeviation(xSlice)
	sd_y, _ := stats.StandardDeviation(xSlice)

	var newXSlice, newYSlice []float64
	for i, _ := range xSlice {
		newXSlice = append(newXSlice, (xSlice[i]-xMean)/sd_x)
		newYSlice = append(newYSlice, (ySlice[i]-yMean)/sd_y)
	}

	var out []Corner
	for i, _ := range corners {
		cor := NewCornerWithR(newXSlice[i], newYSlice[i], corners[i].R)
		out = append(out, cor)
	}
	return out
}

//contains checks a slice of Corners for a particular Corner and returns whether it's there and where it is
func contains(s []Corner, c Corner) (bool, int) {
	for i, v := range s {
		if (v.X == c.X) && (v.Y == c.Y) && (v.R == c.R) {
			return true, i
		}
	}
	return false, len(s)
}

//GetCornerList takes in two gradient images (in X and Y) and a window to perform Harris
//corner detection, producing an exhaustive list of corners. The size of the window will be (w*w)
//and the multiplicative value of the window will always be = 1.
func getCornerList(XGrad, YGrad *image.Gray, w int) ([]Corner, error) {

	XX, _ := MultiplyGrays(XGrad, XGrad)
	XY, _ := MultiplyGrays(XGrad, YGrad)
	YY, _ := MultiplyGrays(YGrad, YGrad)

	list := make([]Corner, 0, XX.Bounds().Max.X*XX.Bounds().Max.Y)
	if !sameImgSize(XX, XY) || !sameImgSize(XY, YY) {
		err := errors.New("these images aren't the same size")
		return list, err
	}

	for y := XX.Bounds().Min.Y + (w / 2); y < XX.Bounds().Max.Y-(w/2); y++ {
		for x := XX.Bounds().Min.X + (w / 2); x < XX.Bounds().Max.X-(w/2); x++ {
			rect := image.Rect(x-(w/2), y-(w/2), x+(w/2), y+(w/2))
			sumXX, sumXY, sumYY := GetSum(XX.SubImage(rect).(*image.Gray16)), GetSum(XY.SubImage(rect).(*image.Gray16)), GetSum(YY.SubImage(rect).(*image.Gray16))

			detM := float64((sumXX * sumYY) - (sumXY * sumXY))
			traceM := float64(sumXX + sumYY)
			R := detM - (0.04 * traceM * traceM) //k=0.04 is a standard value
			list = append(list, Corner{float64(x), float64(y), R})
		}
	}
	return list, nil
}

//ThreshCornerList thresholds the list of potential corners found by GetCornerList()
//based on their R score and returns a list of corners with R > t. Larger R = more cornery
func threshCornerList(list []Corner, t float64) []Corner {
	out := make([]Corner, 0, len(list))
	for _, c := range list {
		if c.R > t {
			out = append(out, c)
		}
	}
	return out
}

//SortCornerListByR takes a list of corners and bubble sorts them such that the highest
//R value (most corner-y) is first and the lowest R value is last
func SortCornerListByR(list []Corner) []Corner {
	for i := 0; i < len(list); i++ {
		for j := 0; j < len(list)-i-1; j++ {
			if list[j].R < list[j+1].R {
				list[j], list[j+1] = list[j+1], list[j]
			}
		}
	}
	return list
}

//SortCornerListByX takes a list of corners and bubble sorts them such that the lowest value of x
//(leftmost) is first and the highest is last
func SortCornerListByX(list []Corner) []Corner {
	for i := 0; i < len(list); i++ {
		for j := 0; j < len(list)-i-1; j++ {
			if list[j].X > list[j+1].X {
				list[j], list[j+1] = list[j+1], list[j]
			}
		}
	}
	return list
}

//SortCornerListByY takes a list of corners and bubble sorts them such that the lowest value of y
//(topmost) is first and the highest is last
func SortCornerListByY(list []Corner) []Corner {
	for i := 0; i < len(list); i++ {
		for j := 0; j < len(list)-i-1; j++ {
			if list[j].Y > list[j+1].Y {
				list[j], list[j+1] = list[j+1], list[j]
			}
		}
	}
	return list
}

//SortCornerListByXY takes a list of corners and bubble sorts them such that the lowest value of x+y
//(topleftmost) is first and the highest is last
func SortCornerListByXY(list []Corner) []Corner {
	for i := 0; i < len(list); i++ {
		for j := 0; j < len(list)-i-1; j++ {
			if list[j].X+list[j].Y > list[j+1].X+list[j+1].Y {
				list[j], list[j+1] = list[j+1], list[j]
			}
		}
	}
	return list
}

//TopNCorners inputs a large list of corners and returns a list of corners (length ~= N) that is
//non-maximally suppressed (spaced out, choosing the max) using dist (in pixels)
func topNCorners(list []Corner, N int, dist float64) ([]Corner, error) {
	if N <= 0 {
		err := errors.New("N must be a positive integer.")
		return list, err
	}

	out, save, dontadd := make([]Corner, 0, N), make([]Corner, 0, len(list)), make([]Corner, 0, len(list))
	sorted := SortCornerListByR(list)
	out = append(out, sorted[0])

	for i := 1; i < N; i++ {
		//Saves every corner that's far enough away from what we've seen so far
		for _, c := range sorted {
			fcx, fcy, fdx, fdy := float64(c.X), float64(c.Y), float64(out[len(out)-1].X), float64(out[len(out)-1].Y)
			if here, _ := contains(dontadd, c); ((math.Abs(fcx-fdx) > dist) || (math.Abs(fcy-fdy) > dist)) && !here {
				save = append(save, c)
			} else {
				dontadd = append(dontadd, c)
			}
		}

		_ = copy(sorted, save)
		save = make([]Corner, 0, len(list))

		//If you're gonna just keep putting the same corner in, don't.
		if AreEqual(sorted[0], out[len(out)-1]) {
			return out, nil
		}
		//Pick the new thing from the top (pre sorted)
		out = append(out, sorted[0])
	}

	return out, nil
}

//getCornersFromPic returns a list of corners (of length N) found in the input image using Harris
//corner detection with window size = window
func getCornersFromPic(pic *rimage.Image, window int, N int) []Corner {
	gray := MakeGray(pic)

	//Now we're gonna use a Sobel kernel (starting at (1,1) cuz it's a 3x3) to make
	//a gradient image in both x and y so we can see what those look like
	xker := rimage.GetSobelX()
	yker := rimage.GetSobelY()
	colGradX, _ := rimage.ConvolveGray(gray, &xker, image.Point{1, 1}, rimage.BorderReplicate)
	colGradY, _ := rimage.ConvolveGray(gray, &yker, image.Point{1, 1}, rimage.BorderReplicate)

	//Calculate potential Harris corners and whittle them down
	list, _ := getCornerList(colGradX, colGradY, window)
	newlist := threshCornerList(list, 1000)   //1000 is an empirically selected threshold
	finlist, _ := topNCorners(newlist, N, 10) //10 pixels away from each other

	return finlist
}

//addCornersToPic takes the existing corners in list and draws "color"" pixels on them on
//top of the image pic. Then, it saves the resulting image to the location at loc
//CURRENTLY WILL CAUSE AN ERROR IF THERES A CORNER TOO CLOSE TO EDGE (FIX?)
func addCornersToPic(list []Corner, pic *rimage.Image, color rimage.Color, loc string) {
	//paintin := rimage.NewColor(255,0,0) //hopefully red or at least noticeable
	radius := 3.0
	for _, l := range list {
		for x := -radius; x < radius; x++ {
			for y := -radius; y < radius; y++ {
				pic.SetXY(int(l.X+x), int(l.Y+y), color)
			}
		}
	}
	SaveImage(pic, loc)
}

//AddNumbersToPic takes the input image and corner list and overlays numbers on the image
//that correspond to the index of the corner in the list.
func addNumbersToPic(list []Corner, pic image.Image, color rimage.Color, loc string) {
	//paintin := rimage.NewColor(255,0,0) //hopefully red or at least noticeable
	//radius := 3.0
	b := pic.Bounds()
	m := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(m, m.Bounds(), pic, b.Min, draw.Src)

	newlist := SortCornerListByX(list)

	for i, l := range newlist {
		point := fixed.Point26_6{fixed.I(int(l.X)), fixed.I(int(l.Y))}
		d := &font.Drawer{
			Dst:  m,
			Src:  image.NewUniform(color),
			Face: basicfont.Face7x13,
			Dot:  point,
		}
		d.DrawString(strconv.Itoa(i))
	}
	SaveImage2(m, loc)
}

//GetAndShowCorners is the outward facing function that reads in an image from inloc, puts a
//figure showing the ordered corners at outloc, and returns the list of corners shown
func GetAndShowCorners(inloc, outloc string, N int) []Corner {

	img, _ := rimage.NewImageFromFile(inloc)
	corList := getCornersFromPic(img, 9, N)
	addCornersToPic(corList, img, rimage.NewColor(0, 0, 255), outloc)
	pick8, _ := pickNRandomCorners(corList, N) //pick N corners

	f, _ := os.Open(outloc)
	defer f.Close()
	img2, _, _ := image.Decode(f)

	addNumbersToPic(pick8, img2, rimage.NewColor(0, 255, 255), outloc)

	return pick8
}

//cornerToXYDist calculates the Euclidean distance between a corner and (x,y)
func cornerToXYDist(C Corner, x, y float64) float64 {
	return math.Sqrt(math.Pow(float64(C.X)-x, 2) + math.Pow(float64(C.Y)-y, 2))
}

//pick4RandomCorners will pick 4 corners at random from the list. Error if len(list) <4
func pick4RandomCorners(list []Corner) ([]Corner, error) {
	if len(list) < 4 {
		return list, errors.New("need a long enough input list (>4)")
	}
	//rand.Seed(time.Now().UnixNano())
	rand.Seed(603)
	rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })

	return list[:4], nil
}

//pickNRandomCorners returns a randomly ordered subset of the input list If N<len(list),
//the output list will have len = N. Otherwise the list will remain unchanged
func pickNRandomCorners(list []Corner, N int) ([]Corner, error) {
	if len(list) < N {
		return list, errors.New("need a long enough input list (>4)")
	}
	if len(list) >= N {
		return list, nil
	}
	//rand.Seed(time.Now().UnixNano())
	rand.Seed(60387)
	rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })

	return list[:N], nil
}


//The remainder of the functions on this page are hacky. They are all tied to FindMagicCorner,
//which considers the first corner such that at least 3 corners exist along both the same X-axis and
//same Y-axis. This idea is predicated on the idea that there's a (properly oriented) checkerboard in the image


//FindMagicCorner inputs a slice of corners and finds the one that has 2 other "friends" in the X-axis
//and 2 other "friends" in the Y-axis (+/- 10 pixels wiggle room)
func FindMagicCorner(list []Corner) (Corner, error) {

	list = SortCornerListByX(list)
	Glist := make([]Corner, 0)
	for _, l := range list {
		Xaxcount, Yaxcount := 0, 0
		for i := 0; i < len(list); i++ {
			//Check if on the same x axis as l
			if math.Abs(float64(list[i].Y-l.Y)) < 10 {
				Xaxcount++
			}
			//Check if on the same y axis as l
			if math.Abs(float64(list[i].X-l.X)) < 10 {
				Yaxcount++
			}
		}

		if Xaxcount >= 3 && Yaxcount >= 3 { //probably will include self
			Glist = append(Glist, l)
		}
	}

	if len(Glist) == 0 {
		err := errors.New("didn't find a magic corner :(")
		return list[0], err
	}

	min := math.Inf(1)
	saveidx := 0
	for i, g := range Glist {
		if float64(g.X+g.Y) < min {
			saveidx = i
			min = float64(g.X + g.Y)
		}
	}
	return Glist[saveidx], nil
}

//RemoveBadCorners inputs a slice of corners and outputs a list of corners such that each corner in the
//output list has an x+y value larger than the "magic" corner
func RemoveBadCorners(list []Corner) ([]Corner, error) {
	newlist := SortCornerListByXY(list)
	C, err := FindMagicCorner(newlist)
	if err != nil {
		return newlist, err
	}
	_, idx := contains(newlist, C)
	return newlist[idx:], nil
}

//Picks 4 points (corners) on the image by starting with the "magic" corner (1)
//and going as far right(2) and down(3) as possible. The last point is closest to the
//X of pt2 and the Y of pt3. Hopefully this will kinda box out the checkerboard.
func pick4Corners(list []Corner) ([]Corner, error) {
	if len(list) < 4 {
		return list, errors.New("need a long enough input list (>4)")
	}
	C1, err := FindMagicCorner(list)
	if err != nil {
		return list, errors.New("couldn't get a good starting point :(")
	}

	list2 := make([]Corner, 0)
	list3 := make([]Corner, 0)
	for _, l := range list {
		if math.Abs(float64(C1.Y-l.Y)) < 20 {
			list2 = append(list2, l)
		}
		if math.Abs(float64(C1.X-l.X)) < 20 {
			list3 = append(list3, l)
		}
	}
	list2 = SortCornerListByX(list2)
	list3 = SortCornerListByY(list3)
	C2 := list2[len(list2)-1]
	C3 := list3[len(list3)-1]

	var C4 Corner
	var newdist float64
	dist := math.Inf(1)
	for _, l := range list {
		newdist = cornerToXYDist(l, float64(C2.X), float64(C3.Y))
		if newdist < dist {
			dist = newdist
			C4 = l
		}
	}
	return []Corner{C1, C2, C3, C4}, nil
}
