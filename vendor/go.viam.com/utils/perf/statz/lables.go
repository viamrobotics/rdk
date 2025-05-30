package statz

import (
	"strconv"

	"github.com/edaniels/golog"
	"go.opencensus.io/tag"
)

// Label holds the metadata associated for each label.
type Label struct {
	Name        string
	Description string
}

// const values for true/false to avoid string creation on each metric record.
const (
	boolValueTrue  string = "true"
	boolValueFalse string = "false"
)

type labelContraint interface {
	~int64 | ~uint64 | ~string | ~bool
}

func labelsToStringSlice(vList ...interface{}) []string {
	strs := make([]string, 0, len(vList))
	for _, v := range vList {
		strs = append(strs, labelToString(v))
	}
	return strs
}

func labelToString(v interface{}) string {
	switch p := v.(type) {
	case string:
		return p
	case int64:
		return strconv.FormatInt(p, 10)
	case uint64:
		return strconv.FormatUint(p, 10)
	case bool:
		if p {
			return boolValueTrue
		}
		return boolValueFalse
	default:
		golog.Global().Fatalf("Invalid type to string, should never happen with the type contraints defined.")
		return ""
	}
}

func tagKeysFromConfig(cfg *MetricConfig) []tag.Key {
	tagKeys := make([]tag.Key, 0, len(cfg.Labels))
	for _, l := range cfg.Labels {
		t, err := tag.NewKey(l.Name)
		if err != nil {
			golog.Global().Fatalf("error creating metric label", err)
			return []tag.Key{}
		}
		tagKeys = append(tagKeys, t)
	}
	return tagKeys
}
