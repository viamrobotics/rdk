//go:build !no_cgo

package rimage

import (
	"image"
	"image/color"
	"math"
	"sort"

	"github.com/disintegration/imaging"
	"github.com/golang/geo/r2"
	"github.com/pkg/errors"
	"gonum.org/v1/gonum/floats"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"

	"go.viam.com/rdk/utils"
)

// AverageColorAndStats returns avg color and avg distances to avg color.
func (i *Image) AverageColorAndStats(p image.Point, radius int) (Color, float64) {
	avg := i.AverageColor(p, 1)

	total := 0.0
	num := 0.0

	for xx := utils.MaxInt(0, p.X-radius); xx < utils.MinInt(p.X+radius, i.width); xx++ {
		for yy := utils.MaxInt(0, p.Y-radius); yy < utils.MinInt(p.Y+radius, i.height); yy++ {
			num++

			myColor := i.Get(image.Point{xx, yy})
			myDistance := avg.Distance(myColor)
			total += myDistance
		}
	}

	avgDistance := total / num

	return avg, avgDistance
}

// AverageColor returns the average color about a certain point.
func (i *Image) AverageColor(p image.Point, radius int) Color {
	return i.AverageColorXY(p.X, p.Y, radius)
}

// AverageColorXY returns the average color about a certain point.
func (i *Image) AverageColorXY(x, y, radius int) Color {
	h := 0.0
	s := 0.0
	v := 0.0

	num := 0.0

	for X := x - radius; X <= x+radius; X++ {
		for Y := y - radius; Y <= y+radius; Y++ {
			if X < 0 || Y < 0 || X >= i.width || Y >= i.height {
				continue
			}

			data := i.Get(image.Point{X, Y})
			H, S, V := data.HsvNormal()
			h += H
			s += S
			v += V

			num++
		}
	}

	return NewColorFromHSV(h/num, s/num, v/num)
}

// InterestingPixels TODO
// TODO(erh): this and SimpleEdgeDetection are super similar, we shouldn't have both probably? or if we do need better names.
func (i *Image) InterestingPixels(t float64) *image.Gray {
	out := image.NewGray(i.Bounds())

	for x := 0; x < i.Width(); x += 3 {
		for y := 0; y < i.Height(); y += 3 {
			_, avgDistance := i.AverageColorAndStats(image.Point{x + 1, y + 1}, 1)

			clr := color.Gray{0}
			if avgDistance > t {
				clr = color.Gray{255}
			}

			for a := 0; a < 3; a++ {
				for b := 0; b < 3; b++ {
					xx := x + a
					yy := y + b
					out.SetGray(xx, yy, clr)
				}
			}
		}
	}

	return out
}

// SimpleEdgeDetection TODO.
func SimpleEdgeDetection(img *Image, t1, blur float64) (*image.Gray, error) {
	img = ConvertImage(imaging.Blur(img, blur))

	out := image.NewGray(img.Bounds())

	for y := 0; y < img.Bounds().Max.Y; y++ {
		for x := 0; x < img.Bounds().Max.X-1; x++ {
			c0 := img.GetXY(x, y)
			c1 := img.GetXY(x+1, y)

			if c0.DistanceLab(c1) >= t1 {
				out.SetGray(x, y, color.Gray{255})
			} else {
				out.SetGray(x, y, color.Gray{0})
			}
		}
	}

	for x := 0; x < img.Bounds().Max.X; x++ {
		for y := 0; y < img.Bounds().Max.Y-1; y++ {
			c0 := img.GetXY(x, y)
			c1 := img.GetXY(x, y+1)

			if c0.DistanceLab(c1) >= t1 {
				out.SetGray(x, y, color.Gray{255})
			}
		}
	}

	return out, nil
}

// CountBrightSpots TODO.
func CountBrightSpots(img *image.Gray, center image.Point, radius int, threshold uint8) int {
	num := 0

	for x := center.X - radius; x < center.X+radius; x++ {
		for y := center.Y - radius; y < center.Y+radius; y++ {
			d := img.GrayAt(x, y)
			if d.Y >= threshold {
				num++
			}
		}
	}

	return num
}

// BilinearInterpolationColor approximates the Color value between pixels according to a bilinear
// interpolation. A nil return value means the interpolation is out of bounds.
func BilinearInterpolationColor(pt r2.Point, img *Image) *Color {
	width, height := float64(img.Width()), float64(img.Height())
	if pt.X < 0 || pt.Y < 0 || pt.X > width-1 || pt.Y > height-1 { // point out of bounds - skip it
		return nil
	}
	xmin := int(math.Floor(pt.X))
	xmax := int(math.Ceil(pt.X))
	ymin := int(math.Floor(pt.Y))
	ymax := int(math.Ceil(pt.Y))
	// get color values
	c00 := img.GetXY(xmin, ymin)
	c10 := img.GetXY(xmax, ymin)
	c01 := img.GetXY(xmin, ymax)
	c11 := img.GetXY(xmax, ymax)
	// calculate weights
	area := float64((xmax - xmin) * (ymax - ymin))
	if area == 0.0 { // exactly on a pixel
		result := img.GetXY(int(pt.X), int(pt.Y))
		return &result
	}
	w00 := ((float64(xmax) - pt.X) * (float64(ymax) - pt.Y)) / area
	w10 := ((pt.X - float64(xmin)) * (float64(ymax) - pt.Y)) / area
	w01 := ((float64(xmax) - pt.X) * (pt.Y - float64(ymin))) / area
	w11 := ((pt.X - float64(xmin)) * (pt.Y - float64(ymin))) / area

	colors := []Color{c00, c01, c10, c11}
	weights := []float64{w00, w01, w10, w11}
	result := AverageColor(colors, weights...)
	return &result
}

// NearestNeighborColor takes the value of the closest point to the intermediate pixel.
func NearestNeighborColor(pt r2.Point, img *Image) *Color {
	width, height := float64(img.Width()), float64(img.Height())
	if pt.X < 0 || pt.Y < 0 || pt.X > width-1 || pt.Y > height-1 { // point out of bounds - skip it
		return nil
	}
	x := int(math.Round(pt.X))
	y := int(math.Round(pt.Y))
	// get color value
	result := img.GetXY(x, y)
	return &result
}

// Rotate rotates the image clockwise by a certain amount of degrees.
func (i *Image) Rotate(amount int) *Image {
	if amount != 180 {
		// made this a panic
		panic("rimage.Image can only rotate 180 degrees right now")
	}

	i2 := NewImage(i.width, i.height)

	k := 0
	for y := 0; y < i.height; y++ {
		for x := 0; x < i.width; x++ {
			val := i.GetXY(i.width-1-x, i.height-1-y)
			i2.data[k] = val

			k++
		}
	}

	return i2
}

// EdgeDetector defines a way to detect edges within an image.
type EdgeDetector interface {
	// DetectEdges detects edges in the given image represented in a grayscale image
	// returns either a map of edges with magnitude or probability or a binary image
	DetectEdges(*Image, ...float64) *image.Gray
	// Edge binary image
	EdgeMap(*Image, ...float64) *Image
}

// CannyEdgeDetector TODO.
type CannyEdgeDetector struct {
	highRatio, lowRatio float64
	preprocessImage     bool
}

// NewCannyDericheEdgeDetector TODO.
func NewCannyDericheEdgeDetector() *CannyEdgeDetector {
	return &CannyEdgeDetector{0.8, 0.33, false}
}

// NewCannyDericheEdgeDetectorWithParameters creates a new Canny edge detector with user provided parameters.
func NewCannyDericheEdgeDetectorWithParameters(hiRatio, loRatio float64, preproc bool) *CannyEdgeDetector {
	return &CannyEdgeDetector{hiRatio, loRatio, preproc}
}

// DetectEdges TODO.
func (cd *CannyEdgeDetector) DetectEdges(img *Image, blur float64) (*image.Gray, error) {
	imgGradient, err := ForwardGradient(img, blur, cd.preprocessImage)
	if err != nil {
		return nil, err
	}
	nms, err := GradientNonMaximumSuppressionC8(imgGradient.Magnitude, imgGradient.Direction)
	if err != nil {
		return nil, err
	}
	low, high, err := GetHysteresisThresholds(imgGradient.Magnitude, nms, cd.highRatio, cd.lowRatio)
	if err != nil {
		return nil, err
	}
	edges, err := EdgeHysteresisFiltering(imgGradient.Magnitude, low, high)
	if err != nil {
		return nil, err
	}
	return edges, nil
}

// Luminance computes the luminance value from the R,G and B values.
// It is defined as (299*R + 587*G + 114*B) / 1000 in order to avoid floating point math issue b/w different
// architectures
// Formula from : https://en.wikipedia.org/wiki/Grayscale#Converting_color_to_grayscale - luma coding.
func Luminance(aColor Color) float64 {
	r, g, b := aColor.RGB255()

	// need to convert uint32 to float64
	return (299*float64(r) + 587*float64(g) + 114*float64(b)) / 1000
}

// ImageGradient TODO.
type ImageGradient struct {
	GradX, GradY         *mat.Dense
	Magnitude, Direction *mat.Dense
}

// ForwardGradient computes the forward gradients in X and Y direction of an image in the Lab space
// Returns: gradient in x direction, gradient in y direction, its magnitude and direction at each pixel in a dense mat,
// and an error.
func ForwardGradient(img *Image, blur float64, preprocess bool) (ImageGradient, error) {
	if preprocess {
		img = ConvertImage(imaging.Blur(img, blur))
	}
	// allocate output matrices
	gradX := mat.NewDense(img.Height(), img.Width(), nil)
	gradY := mat.NewDense(img.Height(), img.Width(), nil)
	magX := mat.NewDense(img.Height(), img.Width(), nil)
	magY := mat.NewDense(img.Height(), img.Width(), nil)
	mag := mat.NewDense(img.Height(), img.Width(), nil)
	direction := mat.NewDense(img.Height(), img.Width(), nil)
	// Compute forward gradient in X direction and its square for magnitude
	for y := 0; y < img.Bounds().Max.Y; y++ {
		for x := 0; x < img.Bounds().Max.X-1; x++ {
			c0 := Luminance(img.GetXY(x, y))
			c1 := Luminance(img.GetXY(x+1, y))
			d := c1 - c0
			gradX.Set(y, x, d)

			magX.Set(y, x, d*d)
		}
	}
	// Compute forward gradient in Y direction and its square
	for x := 0; x < img.Bounds().Max.X; x++ {
		for y := 0; y < img.Bounds().Max.Y-1; y++ {
			c0 := Luminance(img.GetXY(x, y))
			c1 := Luminance(img.GetXY(x, y+1))
			d := c1 - c0
			gradY.Set(y, x, d)
			magY.Set(y, x, d*d)
		}
	}
	// squared norm of forward gradient
	mag.Add(magX, magY)
	// magnitude of forward gradient
	mag.Apply(func(i, j int, v float64) float64 { return math.Sqrt(v) }, mag)
	// get direction of gradient at each pixel
	for x := 0; x < img.Bounds().Max.X; x++ {
		for y := 0; y < img.Bounds().Max.Y; y++ {
			gx := gradX.At(y, x)
			gy := gradY.At(y, x)
			if gx != 0 {
				direction.Set(y, x, math.Atan2(gy, gx))
			} else {
				direction.Set(y, x, 0)
			}
		}
	}

	return ImageGradient{gradX, gradY, mag, direction}, nil
}

// SobelColorGradient takes in a color image, approximates the gradient in the X and Y direction at every pixel
// creates a  vector in polar form, and returns a vector field.
func SobelColorGradient(img *Image) (VectorField2D, error) {
	width, height := img.Width(), img.Height()
	maxMag := 0.0
	g := make([]Vec2D, 0, width*height)
	sobel := sobelColorFilter()
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			point := image.Point{x, y}
			sX, sY := sobel(point, img)
			mag, dir := getMagnitudeAndDirection(sX, sY)
			g = append(g, Vec2D{mag, dir})
			maxMag = math.Max(math.Abs(mag), maxMag)
		}
	}
	vf := VectorField2D{width, height, g, maxMag}
	return vf, nil
}

// MatrixPixelPoint defines a point in a matrix.
type MatrixPixelPoint struct {
	I, J int
}

// GradientNonMaximumSuppressionC8 computes the non maximal suppression of edges in Connectivity 8.
// For each pixel, it checks if at least one the two pixels in the current gradient direction has a greater magnitude
// than the current pixel.
func GradientNonMaximumSuppressionC8(mag, direction *mat.Dense) (*mat.Dense, error) {
	r, c := mag.Dims()
	nms := mat.NewDense(r, c, nil)
	for j := 1; j < c-1; j++ {
		for i := 1; i < r-1; i++ {
			angle := direction.At(i, j)
			// for easier calculation, get positive angle value
			if angle < 0 {
				angle += math.Pi
			}
			// Compute bin of gradient direction at current pixel
			rangle := int(math.Round(angle/(math.Pi/4) + 0.5))
			// Compare pixel values in the gradient direction with current pixel value
			magVal := mag.At(i, j)
			cond1 := (rangle%4 == 0) && (mag.At(i, j-1) > magVal || mag.At(i, j+1) > magVal)
			cond2 := rangle%4 == 1 && (mag.At(i-1, j+1) > magVal || mag.At(i+1, j-1) > magVal)
			cond3 := rangle%4 == 2 && (mag.At(i-1, j) > magVal || mag.At(i+1, j) > magVal)
			cond4 := rangle%4 == 3 && (mag.At(i-1, j-1) > magVal || mag.At(i+1, j+1) > magVal)
			// if current pixel value if greater than the ones in the gradient direction, current pixel is a local
			// maximum
			if !(cond1 || cond2 || cond3 || cond4) {
				nms.Set(i, j, mag.At(i, j))
			}
		}
	}

	return nms, nil
}

// GetHysteresisThresholds computes the low and high thresholds for the Canny Hysteresis Edge Thresholding.
// John Canny said in his paper "A Computational Approach to Edge Detection" that "The ratio of the
// high to low threshold in the implementation is in the range two or three to one."
// So, in this implementation, we should choose tlow ~= 0.5 or 0.33333.
// A good value for thigh is around 0.8.
func GetHysteresisThresholds(mag, nms *mat.Dense, ratioHigh, ratioLow float64) (float64, float64, error) {
	var low, high float64
	x := make([]float64, len(mag.RawMatrix().Data))
	r, c := mag.Dims()
	// Get gradient magnitude values as a slice of float64 to compute the histogram
	copy1 := copy(x, mag.RawMatrix().Data)

	if copy1 == 0 {
		err := errors.New("the slice copy was not achieved")
		return 0, 0, err
	}
	sort.Float64s(x)
	// Compute histogram of magnitude image
	max := floats.Max(x)
	// Get one bin per possible pixel value
	nBins := int(math.Round(max))
	// Increase the maximum divider so that the maximum value of x is contained
	// within the last bucket.
	max++
	// Create histogram dividers
	dividers := make([]float64, nBins+1)
	floats.Span(dividers, 0, max)
	hist := stat.Histogram(nil, dividers, x, nil)

	// Remove zeros values from histogram
	hist = hist[1:]
	// Non Zero Pixels in mag
	nNonZero := floats.Sum(hist)
	// Compute high threshold
	high = nNonZero * ratioHigh * 100 / float64(r*c)
	// Compute low threshold
	low = high*ratioLow + 0.5

	return low, high, nil
}

// GetConnectivity8Neighbors return the pixel coordinates of the neighbors of a pixel (i,j) in connectivity 8;
// Returns only the pixel within the image bounds.
// Connectivity 8 :
//
//	.   .   .
//	.   o   .
//	.   .   .
func GetConnectivity8Neighbors(i, j, r, c int) []MatrixPixelPoint {
	neighbors := make([]MatrixPixelPoint, 0, 8)
	if i-1 > 0 && j-1 > 0 {
		neighbors = append(neighbors, MatrixPixelPoint{i - 1, j - 1})
	}
	if i-1 > 0 {
		neighbors = append(neighbors, MatrixPixelPoint{i - 1, j})
	}
	if i-1 > 0 && j+1 < c {
		neighbors = append(neighbors, MatrixPixelPoint{i - 1, j + 1})
	}
	if j-1 > 0 {
		neighbors = append(neighbors, MatrixPixelPoint{i, j - 1})
	}
	if j+1 < c {
		neighbors = append(neighbors, MatrixPixelPoint{i, j + 1})
	}
	if i+1 < r && j-1 > 0 {
		neighbors = append(neighbors, MatrixPixelPoint{i + 1, j - 1})
	}
	if i+1 < r {
		neighbors = append(neighbors, MatrixPixelPoint{i + 1, j})
	}
	if i+1 < r && j+1 < c {
		neighbors = append(neighbors, MatrixPixelPoint{i + 1, j + 1})
	}
	return neighbors
}

// EdgeHysteresisFiltering performs the Non Maximum Suppressed edges hysteresis filtering
// every pixel whose value is above high is preserved.
// Any pixel whose value falls into [low, high] and that is connected to a high value pixel is preserved as well
// as all pixel whose value is below low is set to zero.
// This allows to remove weak edges but preserves edges that are strong or partially strong.
func EdgeHysteresisFiltering(mag *mat.Dense, low, high float64) (*image.Gray, error) {
	r, c := mag.Dims()
	visited := map[MatrixPixelPoint]bool{}
	edges := image.NewGray(image.Rect(0, 0, c, r))
	// Keep edge pixels with strong gradient value
	for j := 0; j < c; j++ {
		for i := 0; i < r; i++ {
			if mag.At(i, j) > high {
				coords := MatrixPixelPoint{i, j}
				visited[coords] = true
			}
		}
	}
	// Keep edge pixels with weak gradient value next to an edge pixel with a strong value
	lastIterationQueue := visited
	for len(lastIterationQueue) > 0 {
		newKeep := map[MatrixPixelPoint]bool{}
		// Iterate through list and print its contents.
		for coords := range lastIterationQueue {
			// coords := e.Value.(MatrixPixelPoint)
			neighbors := GetConnectivity8Neighbors(coords.I, coords.J, r, c)
			for _, nb := range neighbors {
				isPixelVisited := visited[nb]
				if mag.At(nb.I, nb.J) > low && !isPixelVisited {
					newKeep[nb] = true
					visited[nb] = true
				}
			}
		}
		lastIterationQueue = newKeep
	}
	// Fill out image
	for coords := range visited {
		edges.Set(coords.J, coords.I, color.Gray{255})
	}
	return edges, nil
}

// ImageToYCbCrForTesting converts an image to YCbCr. It is only to be used for testing.
func ImageToYCbCrForTesting(dst *image.YCbCr, src image.Image) {
	if dst == nil {
		panic("dst can't be nil")
	}

	yuvImg, ok := src.(*image.YCbCr)
	if ok {
		*dst = *yuvImg
		return
	}

	bounds := src.Bounds()
	dy := bounds.Dy()
	dx := bounds.Dx()
	flat := dy * dx

	if len(dst.Y)+len(dst.Cb)+len(dst.Cr) < 3*flat {
		i0 := 1 * flat
		i1 := 2 * flat
		i2 := 3 * flat
		if cap(dst.Y) < i2 {
			dst.Y = make([]uint8, i2)
		}
		dst.Y = dst.Y[:i0]
		dst.Cb = dst.Y[i0:i1]
		dst.Cr = dst.Y[i1:i2]
	}
	dst.SubsampleRatio = image.YCbCrSubsampleRatio444
	dst.YStride = dx
	dst.CStride = dx
	dst.Rect = bounds

	i := 0
	for yi := 0; yi < dy; yi++ {
		for xi := 0; xi < dx; xi++ {
			// TODO(erh): probably try to get the alpha value with something like
			// https://en.wikipedia.org/wiki/Alpha_compositing
			r, g, b, _ := src.At(xi, yi).RGBA()
			yy, cb, cr := color.RGBToYCbCr(uint8(r/256), uint8(g/256), uint8(b/256))
			dst.Y[i] = yy
			dst.Cb[i] = cb
			dst.Cr[i] = cr
			i++
		}
	}
}
