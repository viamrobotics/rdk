package classification

import "strings"

// Postprocessor defines a function that filters/modifies on an incoming array of Classifications.
type Postprocessor func(Classifications) Classifications

// NewScoreFilter returns a function that filters out classifications below a certain confidence
// score.
func NewScoreFilter(conf float64) Postprocessor {
	return func(in Classifications) Classifications {
		out := make(Classifications, 0, len(in))
		for _, c := range in {
			if c.Score() >= conf {
				out = append(out, c)
			}
		}
		return out
	}
}

// NewLabelFilter returns a function that filters out classifications without one of the chosen labels.
// Does not filter when input is empty.
func NewLabelFilter(labels map[string]interface{}) Postprocessor {
	return func(in Classifications) Classifications {
		if len(labels) < 1 {
			return in
		}
		out := make(Classifications, 0, len(in))
		for _, c := range in {
			if _, ok := labels[strings.ToLower(c.Label())]; ok {
				out = append(out, c)
			}
		}
		return out
	}
}

// NewLabelConfidenceFilter returns a function that filters out classifications based on label map.
// Does not filter when input is empty.
func NewLabelConfidenceFilter(labels map[string]float64) Postprocessor {
	// ensure all the label names are lower case
	theLabels := make(map[string]float64)
	for name, conf := range labels {
		theLabels[strings.ToLower(name)] = conf
	}
	return func(in Classifications) Classifications {
		if len(theLabels) < 1 {
			return in
		}
		out := make(Classifications, 0, len(in))
		for _, c := range in {
			if conf, ok := theLabels[strings.ToLower(c.Label())]; ok {
				if c.Score() >= conf {
					out = append(out, c)
				}
			}
		}
		return out
	}
}
