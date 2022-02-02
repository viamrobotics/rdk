package objectdetection

// Filter defines a function that filters on an incoming array of Detections.
type Filter func([]Detection) []Detection

// NewAreaFilter returns a Filter function that filters out detections below a certain area.
func NewAreaFilter(area int) Filter {
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
