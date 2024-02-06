// Package inference allows users to do inference through tflite (tf, pytorch, etc in the future)
package inference

import (
	"github.com/pkg/errors"
	"go.viam.com/rdk/ml"
)

// MLModel represents a trained machine learning model.
type MLModel interface {
	// Infer takes an already ordered input tensor as an array,
	// and makes an inference on the model, returning an output tensor map
	Infer(inputTensors ml.Tensors) (ml.Tensors, error)

	// Metadata gets the entire model metadata structure from file
	Metadata() (interface{}, error)

	// Close closes the model and interpreter that allows inferences to be made, opens up space in memory.
	// All models must be closed when done using
	Close() error
}

// FailedToLoadError is the default error message for when expected resources for inference fail to load.
func FailedToLoadError(name string) error {
	return errors.Errorf("failed to load %s", name)
}

// FailedToGetError is the default error message for when expected information will be fetched fails.
func FailedToGetError(name string) error {
	return errors.Errorf("failed to get %s", name)
}

// MetadataDoesNotExistError returns a metadata does not exist error.
func MetadataDoesNotExistError() error {
	return errors.New("metadata does not exist")
}
