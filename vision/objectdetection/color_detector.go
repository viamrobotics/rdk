package objectdetection

import (
	"image"

	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage"
)

// ColorDetectorConfig specifies the fields necessary for creating a color detector.
type ColorDetectorConfig struct {
	SegmentSize       int     `json:"segment_size_px"`
	HueTolerance      float64 `json:"hue_tolerance_pct"`
	SaturationCutoff  float64 `json:"saturation_cutoff_pct"`
	ValueCutoff       float64 `json:"value_cutoff_pct"`
	DetectColorString string  `json:"detect_color"` // hex string "#RRGGBB"
}

// NewColorDetector is a detector that identifies objects based on color.
// It takes in a hue value between 0 and 360, and then defines a valid range around the hue of that color
// based on the tolerance. The color is considered valid if the pixel is between (hue - tol) <= color <= (hue + tol).
func NewColorDetector(cfg *ColorDetectorConfig) (Detector, error) {
	col, err := rimage.NewColorFromHex(cfg.DetectColorString)
	if err != nil {
		return nil, err
	}
	hue, _, _ := col.HsvNormal()
	tol := cfg.HueTolerance
	if tol > 1.0 || tol < 0.0 {
		return nil, errors.Errorf("tolerance must be between 0.0 and 1.0. Got %.5f", tol)
	}


	sat := cfg.SaturationCutoff
	if sat <= 0.0 {
		sat = 0.2
	}
	val := cfg.SaturationCutoff
	if val <= 0.0 {
		val = 0.3
	}

	var valid validPixelFunc
	if tol == 1.0 {
		valid = makeValidColorFunction(0, 360, sat, val)
	} else {
		tol = (tol / 2.) * 360.0 // change from percent to degrees
		hiValid := hue + tol
		if hiValid >= 360. {
			hiValid -= 360.
		}
		loValid := hue - tol
		if loValid < 0. {
			loValid += 360.
		}
		valid = makeValidColorFunction(loValid, hiValid, sat, val)
	}
	cd := connectedComponentDetector{valid, hueToString(hue)}
	// define the filter
	segmentSize := 5000 // default value
	if cfg.SegmentSize != 0 {
		segmentSize = cfg.SegmentSize
	}
	filt := NewAreaFilter(segmentSize)
	// build the detector pipeline
	det, err := Build(nil, cd.Inference, filt)
	if err != nil {
		return nil, err
	}
	sortedDet, err := Build(nil, det, SortByArea())
	if err != nil {
		return nil, err
	}

	return sortedDet, nil
}

func hueToString(hue float64) string {
	hueInt := int(hue) % 360
	switch {
	case hueInt < 15 || hueInt >= 345:
		return "red"
	case hueInt >= 15 && hueInt < 45:
		return "orange"
	case hueInt >= 45 && hueInt < 75:
		return "yellow"
	case hueInt >= 75 && hueInt < 105:
		return "lime-green"
	case hueInt >= 105 && hueInt < 135:
		return "green"
	case hueInt >= 135 && hueInt < 165:
		return "green-blue"
	case hueInt >= 165 && hueInt < 195:
		return "cyan"
	case hueInt >= 195 && hueInt < 225:
		return "light-blue"
	case hueInt >= 225 && hueInt < 255:
		return "blue"
	case hueInt >= 255 && hueInt < 285:
		return "violet"
	case hueInt >= 285 && hueInt < 315:
		return "magenta"
	case hueInt >= 315 && hueInt < 345:
		return "rose"
	default:
		return "impossible"
	}
}

func makeValidColorFunction(loValid, hiValid, sat, val float64) validPixelFunc {
	valid := func(v float64) bool { return v == loValid }
	if hiValid > loValid {
		valid = func(v float64) bool { return v <= hiValid && v >= loValid }
	} else if loValid > hiValid {
		valid = func(v float64) bool { return v <= hiValid || v >= loValid }
	}
	// create the ValidPixel function
	return func(img image.Image, pt image.Point) bool {
		c := rimage.NewColorFromColor(img.At(pt.X, pt.Y))
		h, s, v := c.HsvNormal()
		if s < sat {
			return false
		}
		if v < val {
			return false
		}
		return valid(h)
	}
}
