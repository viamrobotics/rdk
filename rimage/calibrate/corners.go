package calibrate

import (
	"image"
	"image/draw"
	"math"
	"math/rand"
	"os"
	"sort"
	"strconv"

	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"go.viam.com/rdk/rimage"
)

// Corner refers to a point on an image with a corner value=R (Harris detection).
type Corner struct {
	X float64
	Y float64
	R float64 // Cornerness
}

// harrisK is the constant empirically selected in Harris corner detection to relate
// the determinant to the trace of the matrix of products.
const harrisK = 0.04

// NewCorner creates a new corner without a value for R.
func NewCorner(x, y float64) Corner {
	return Corner{X: x, Y: y}
}

// NewCornerWithR creates a new corner with an R value.
func NewCornerWithR(x, y float64, r float64) Corner {
	return Corner{X: x, Y: y, R: r}
}

// AreEqual is a simple equality test for corners. If all fields are equal, same corner.
func AreEqual(a, b Corner) bool {
	if a.X == b.X && a.Y == b.Y && a.R == b.R {
		return true
	}
	return false
}

// NormalizeCorners takes in a list of corners and returns a list of corners such that the output list
// has x and y values that are (x - xmean)/std(x).
func NormalizeCorners(corners []Corner) []Corner {
	if len(corners) <= 1 {
		return corners
	}
	xSlice := make([]float64, 0, len(corners))
	ySlice := make([]float64, 0, len(corners))
	for _, c := range corners {
		xSlice = append(xSlice, c.X)
		ySlice = append(ySlice, c.Y)
	}

	xMean, err := stats.Mean(xSlice)
	if err != nil {
		panic(err)
	}
	yMean, err := stats.Mean(ySlice)
	if err != nil {
		panic(err)
	}
	sdX, err := stats.StandardDeviation(xSlice)
	if err != nil {
		panic(err)
	}
	sdY, err := stats.StandardDeviation(xSlice)
	if err != nil {
		panic(err)
	}

	newXSlice := make([]float64, 0, len(xSlice))
	newYSlice := make([]float64, 0, len(ySlice))
	for i := range xSlice {
		newXSlice = append(newXSlice, (xSlice[i]-xMean)/sdX)
		newYSlice = append(newYSlice, (ySlice[i]-yMean)/sdY)
	}

	out := make([]Corner, 0, len(corners))
	for i := range corners {
		cor := NewCornerWithR(newXSlice[i], newYSlice[i], corners[i].R)
		out = append(out, cor)
	}
	return out
}

// contains checks a slice of Corners for a particular Corner and returns whether it's there and where it is.
func contains(s []Corner, c Corner) (bool, int) {
	for i, v := range s {
		if (v.X == c.X) && (v.Y == c.Y) && (v.R == c.R) {
			return true, i
		}
	}
	return false, len(s)
}

// GetCornerList takes in two gradient images (in X and Y) and a window to perform Harris
// corner detection, producing an exhaustive list of corners. The size of the window will be (w*w)
// and the multiplicative value of the window will always be = 1.
func getCornerList(xGrad, yGrad *image.Gray, w int) ([]Corner, error) {
	XX, err := rimage.MultiplyGrays(xGrad, xGrad)
	if err != nil {
		return nil, err
	}
	XY, err := rimage.MultiplyGrays(xGrad, yGrad)
	if err != nil {
		return nil, err
	}
	YY, err := rimage.MultiplyGrays(yGrad, yGrad)
	if err != nil {
		return nil, err
	}

	list := make([]Corner, 0, XX.Bounds().Max.X*XX.Bounds().Max.Y)
	if !rimage.SameImgSize(XX, XY) {
		return nil, errors.Errorf("these images aren't the same size (%d %d) != (%d %d)",
			XX.Bounds().Max.X, XX.Bounds().Max.Y, XY.Bounds().Max.X, XY.Bounds().Max.Y)
	}
	if !rimage.SameImgSize(XY, YY) {
		return nil, errors.Errorf("these images aren't the same size (%d %d) != (%d %d)",
			XY.Bounds().Max.X, XY.Bounds().Max.Y, YY.Bounds().Max.X, YY.Bounds().Max.Y)
	}

	for y := XX.Bounds().Min.Y + (w / 2); y < XX.Bounds().Max.Y-(w/2); y++ {
		for x := XX.Bounds().Min.X + (w / 2); x < XX.Bounds().Max.X-(w/2); x++ {
			rect := image.Rect(x-(w/2), y-(w/2), x+(w/2), y+(w/2))
			sumXX, sumXY, sumYY := rimage.GetGraySum(XX.SubImage(rect).(*image.Gray16)),
				rimage.GetGraySum(XY.SubImage(rect).(*image.Gray16)), rimage.GetGraySum(YY.SubImage(rect).(*image.Gray16))

			detM := float64((sumXX * sumYY) - (sumXY * sumXY))
			traceM := float64(sumXX + sumYY)
			R := detM - (harrisK * traceM * traceM) // k=0.04 is a standard value
			list = append(list, Corner{float64(x), float64(y), R})
		}
	}
	return list, nil
}

// ThreshCornerList thresholds the list of potential corners found by GetCornerList()
// based on their R score and returns a list of corners with R > t. Larger R = more cornery.
func threshCornerList(list []Corner, t float64) []Corner {
	out := make([]Corner, 0, len(list))
	for _, c := range list {
		if c.R > t {
			out = append(out, c)
		}
	}
	return out
}

// SortCornerListByR takes a list of corners and bubble sorts them such that the highest
// R value (most corner-y) is first and the lowest R value is last.
func SortCornerListByR(list []Corner) []Corner {
	sort.Slice(list, func(i, j int) bool {
		return list[i].R > list[j].R
	})
	return list
}

// SortCornerListByX takes a list of corners and bubble sorts them such that the lowest value of x
// (leftmost) is first and the highest is last.
func SortCornerListByX(list []Corner) []Corner {
	sort.Slice(list, func(i, j int) bool {
		return list[i].X < list[j].X
	})
	return list
}

// SortCornerListByY takes a list of corners and bubble sorts them such that the lowest value of y
// (topmost) is first and the highest is last.
func SortCornerListByY(list []Corner) []Corner {
	sort.Slice(list, func(i, j int) bool {
		return list[i].Y < list[j].Y
	})
	return list
}

// SortCornerListByXY takes a list of corners and bubble sorts them such that the lowest value of x+y
// (topleftmost) is first and the highest is last.
func SortCornerListByXY(list []Corner) []Corner {
	sort.Slice(list, func(i, j int) bool {
		return list[i].X+list[i].Y < list[j].X+list[j].Y
	})
	return list
}

// TopNCorners inputs a large list of corners and returns a list of corners (length ~= N) that is
// non-maximally suppressed (spaced out, choosing the max) using dist (in pixels).
func topNCorners(list []Corner, n int, dist float64) ([]Corner, error) {
	if n <= 0 {
		return nil, errors.Errorf("n must be a positive integer, got %d", n)
	}

	out, save, dontadd := make([]Corner, 0, n), make([]Corner, 0, len(list)), make([]Corner, 0, len(list))
	sorted := SortCornerListByR(list)
	out = append(out, sorted[0])

	for i := 1; i < n; i++ {
		// Saves every corner that's far enough away from what we've seen so far
		for _, c := range sorted {
			fcx, fcy, fdx, fdy := c.X, c.Y, out[len(out)-1].X, out[len(out)-1].Y
			if here, _ := contains(dontadd, c); ((math.Abs(fcx-fdx) > dist) || (math.Abs(fcy-fdy) > dist)) && !here {
				save = append(save, c)
			} else {
				dontadd = append(dontadd, c)
			}
		}

		_ = copy(sorted, save)
		save = make([]Corner, 0, len(list))

		// If you're gonna just keep putting the same corner in, don't.
		if AreEqual(sorted[0], out[len(out)-1]) {
			return out, nil
		}
		// Pick the new thing from the top (pre sorted)
		out = append(out, sorted[0])
	}

	return out, nil
}

// getCornersFromPic returns a list of corners (of length n) found in the input image using Harris
// corner detection with window size = window. Input thresh sets a threshold for what is considered a
// corner (empirically ~1000) and spacing sets a min limit for how close output corners can be.
func getCornersFromPic(pic *rimage.Image, window, n int, thresh, spacing float64) ([]Corner, error) {
	gray := rimage.MakeGray(pic)

	// Now we're gonna use a Sobel kernel (starting at (1,1) cuz it's a 3x3) to make
	// a gradient image in both x and y so we can see what those look like
	xker := rimage.GetSobelX()
	yker := rimage.GetSobelY()
	colGradX, err := rimage.ConvolveGray(gray, &xker, image.Point{1, 1}, rimage.BorderReplicate)
	if err != nil {
		return nil, err
	}
	colGradY, err := rimage.ConvolveGray(gray, &yker, image.Point{1, 1}, rimage.BorderReplicate)
	if err != nil {
		return nil, err
	}

	// Calculate potential Harris corners and whittle them down
	list, err := getCornerList(colGradX, colGradY, window)
	if err != nil {
		return nil, err
	}
	newlist := threshCornerList(list, thresh)
	finlist, err := topNCorners(newlist, n, spacing)
	if err != nil {
		return nil, err
	}

	return finlist, nil
}

// addCornersToPic takes the existing corners in list and draws a "color" dot of radius "radius" on top
// of the corners on top of the image pic. Then, it saves the resulting image to the location at loc
// TODO(kj): CURRENTLY WILL CAUSE AN ERROR IF THERES A CORNER TOO CLOSE TO EDGE (FIX?)
func addCornersToPic(list []Corner, pic *rimage.Image, color rimage.Color, radius float64, loc string) error {
	for _, l := range list {
		for x := -radius; x < radius; x++ {
			for y := -radius; y < radius; y++ {
				pic.SetXY(int(l.X+x), int(l.Y+y), color)
			}
		}
	}
	if err := rimage.SaveImage(pic, loc); err != nil {
		return err
	}
	return nil
}

// AddNumbersToPic takes the input image and corner list and overlays numbers on the image
// that correspond to the index of the corner in the list.
func addNumbersToPic(list []Corner, pic image.Image, color rimage.Color, loc string) error {
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
	if err := rimage.SaveImage(m, loc); err != nil {
		return err
	}
	return nil
}

// GetAndShowCorners is the outward facing function that reads in an image from inloc, puts a
// figure showing the ordered corners at outloc, and returns the list of corners shown.
func GetAndShowCorners(inloc, outloc string, n int) ([]Corner, error) {
	img, err := rimage.NewImageFromFile(inloc)
	if err != nil {
		return nil, err
	}
	corList, err := getCornersFromPic(img, 9, n, 1000, 10)
	if err != nil {
		return nil, err
	}

	err = addCornersToPic(corList, img, rimage.NewColor(0, 0, 255), 3, outloc)
	if err != nil {
		return nil, err
	}
	pick8, err := pickNRandomCorners(corList, n) // pick N corners
	if err != nil {
		return nil, err
	}

	f, err := os.Open(outloc) // nolint:gosec
	if err != nil {
		return nil, err
	}

	defer func() { // nolint:gosec
		if err := f.Close(); err != nil {
			panic(err)
		}
	}()

	img2, _, err2 := image.Decode(f)
	if err2 != nil {
		return nil, err
	}
	err = addNumbersToPic(pick8, img2, rimage.NewColor(0, 255, 255), outloc)
	if err != nil {
		return nil, err
	}
	return pick8, nil
}

// pickNRandomCorners returns a randomly ordered subset of the input list If N<len(list),
// the output list will have len = N. Otherwise the list will remain unchanged.
func pickNRandomCorners(list []Corner, n int) ([]Corner, error) {
	if len(list) < n {
		return list, errors.Errorf("need a long enough input list (>4), got %d", len(list))
	}
	if len(list) >= n {
		return list, nil
	}

	rand.Seed(60387) // Deterministic seed for consistency. Once world points are added, these remain corresponding
	rand.Shuffle(len(list), func(i, j int) { list[i], list[j] = list[j], list[i] })

	return list[:n], nil
}

// The remainder of the functions on this page are hacky. They are all tied to FindMagicCorner,
// which considers the first corner such that at least 3 corners exist along both the same X-axis and
// same Y-axis. This idea is predicated on the idea that there's a (properly oriented) checkerboard in the image

// FindMagicCorner inputs a slice of corners and finds the one that has 2 other "friends" in the X-axis
// and 2 other "friends" in the Y-axis (+/- 10 pixels wiggle room).
func FindMagicCorner(list []Corner) (Corner, error) {
	list = SortCornerListByX(list)
	Glist := make([]Corner, 0)
	for _, l := range list {
		Xaxcount, Yaxcount := 0, 0
		for i := 0; i < len(list); i++ {
			// Check if on the same x axis as l
			if math.Abs(list[i].Y-l.Y) < 10 {
				Xaxcount++
			}
			// Check if on the same y axis as l
			if math.Abs(list[i].X-l.X) < 10 {
				Yaxcount++
			}
		}

		if Xaxcount >= 3 && Yaxcount >= 3 { // probably will include self
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
		if g.X+g.Y < min {
			saveidx = i
			min = g.X + g.Y
		}
	}
	return Glist[saveidx], nil
}

// RemoveBadCorners inputs a slice of corners and outputs a list of corners such that each corner in the
// output list has an x+y value larger than the "magic" corner.
func RemoveBadCorners(list []Corner) ([]Corner, error) {
	newlist := SortCornerListByXY(list)
	C, err := FindMagicCorner(newlist)
	if err != nil {
		return newlist, err
	}
	_, idx := contains(newlist, C)
	return newlist[idx:], nil
}
