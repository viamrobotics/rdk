package ml

import (
	"strconv"
	"strings"
	"sync"

	"github.com/montanaflynn/stats"
	"github.com/pkg/errors"

	"go.viam.com/rdk/vision/classification"
)

const classifierProbabilityName = "probability"

// FormatClassificationOutputs formats the output tensors from a model into classifications.
func FormatClassificationOutputs(
	outNameMap *sync.Map, outMap Tensors, labels []string,
) (classification.Classifications, error) {
	// check if output tensor name that classifier is looking for is already present
	// in the nameMap. If not, find the probability name, and cache it in the nameMap
	pName, ok := outNameMap.Load(classifierProbabilityName)
	if !ok {
		_, ok := outMap[classifierProbabilityName]
		if !ok {
			if len(outMap) == 1 {
				for name := range outMap { //  only 1 element in map, assume its probabilities
					outNameMap.Store(classifierProbabilityName, name)
					pName = name
				}
			}
		} else {
			outNameMap.Store(classifierProbabilityName, classifierProbabilityName)
			pName = classifierProbabilityName
		}
	}
	probabilityName, ok := pName.(string)
	if !ok {
		return nil, errors.Errorf("name map did not store a string of the tensor name, but an object of type %T instead", pName)
	}
	data, ok := outMap[probabilityName]
	if !ok {
		return nil, errors.Errorf("no tensor named 'probability' among output tensors [%s]", strings.Join(TensorNames(outMap), ", "))
	}
	probs, err := ConvertToFloat64Slice(data.Data())
	if err != nil {
		return nil, err
	}
	confs := checkClassificationScores(probs)
	if labels != nil && len(labels) != len(confs) {
		return nil, errors.Errorf("length of output (%d) expected to be length of label list (%d)", len(confs), len(labels))
	}
	classifications := make(classification.Classifications, 0, len(confs))
	for i := 0; i < len(confs); i++ {
		if labels == nil {
			classifications = append(classifications, classification.NewClassification(confs[i], strconv.Itoa(i)))
		} else {
			if i >= len(labels) {
				return nil, errors.Errorf("cannot access label number %v from label file with %v labels", i, len(labels))
			}
			classifications = append(classifications, classification.NewClassification(confs[i], labels[i]))
		}
	}
	return classifications, nil
}

// checkClassification scores ensures that the input scores (output of classifier)
// will represent confidence values (from 0-1).
func checkClassificationScores(in []float64) []float64 {
	if len(in) > 1 {
		for _, p := range in {
			if p < 0 || p > 1 { // is logit, needs softmax
				confs := softmax(in)
				return confs
			}
		}
		return in // no need to softmax
	}
	// otherwise, this is a binary classifier
	if in[0] < -1 || in[0] > 1 { // needs sigmoid
		out, err := stats.Sigmoid(in)
		if err != nil {
			return in
		}
		return out
	}
	return in // no need to sigmoid
}
