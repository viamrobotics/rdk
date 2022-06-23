package objectdetection

// Postprocessor defines a function that filters/modifies on an incoming array of Detections.
type Postprocessor func([]Detection) []Detection

// NewAreaFilter returns a function that filters out detections below a certain area.
func NewAreaFilter(area int) Postprocessor {
	return func(in []Detection) []Detection {
		out := make([]Detection, 0, len(in))
		for _, d := range in {
			if d.BoundingBox().Dx()*d.BoundingBox().Dy() >= area {
				out = append(out, d)
			}
		}
		return out
	}
}

// NewScoreFilter returns a function that filters out detections below a certain confidence.
func NewScoreFilter(conf float64) Postprocessor {
	return func(in []Detection) []Detection {
		out := make([]Detection, 0, len(in))
		for _, d := range in {
			if d.Score() >= conf {
				out = append(out, d)
			}
		}
		return out
	}
}
