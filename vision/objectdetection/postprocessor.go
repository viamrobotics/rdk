package objectdetection

import "sort"

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

// NewLabelFilter returns a function that filters out detections without one of the chosen labels.
func NewLabelFilter(labels map[string]interface{}) Postprocessor {
	return func(in []Detection) []Detection {
		out := make([]Detection, 0, len(in))
		for _, d := range in {
			if _, ok := labels[d.Label()]; ok {
				out = append(out, d)
			}
		}
		return out
	}
}

// SortByArea returns a function that sorts the list of detections by area (largest first).
func SortByArea() Postprocessor {
	return func(in []Detection) []Detection {
		sort.Slice(in, func(i, j int) bool {
			return in[i].BoundingBox().Dx()*in[i].BoundingBox().Dy() > in[j].BoundingBox().Dx()*in[j].BoundingBox().Dy()
		})
		return in
	}
}
