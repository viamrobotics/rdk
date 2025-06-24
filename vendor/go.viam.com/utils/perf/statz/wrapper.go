package statz

import (
	"github.com/edaniels/golog"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

// opencensusStatsData holds the generated open census types (views, tags) for each metric.
type opencensusStatsData struct {
	View      *view.View
	labelKeys []tag.Key
}

// labelsToMutations creates the opencensus Mutations for each label value already converted to a string. Joins the pre-computed tags
// with the string values.
func (sd *opencensusStatsData) labelsToMutations(labels []string) []tag.Mutator {
	if len(labels) != len(sd.labelKeys) {
		golog.Global().Panic("Should never happen where the label lengths do not match")
		return []tag.Mutator{}
	}

	mutations := make([]tag.Mutator, 0, len(labels))
	for i, f := range labels {
		t := sd.labelKeys[i]
		mutation := tag.Upsert(t, f)
		mutations = append(mutations, mutation)
	}

	return mutations
}
