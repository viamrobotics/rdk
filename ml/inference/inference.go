// package inference allows users to do inference through tflite (tf, pytorch, etc in the future)
package inference

// #cgo LDFLAGS: -L/Users/alexiswei/Documents/repos/tensorflow/bazel-bin/tensorflow/lite/c
// #cgo CFLAGS: -I/Users/alexiswei/Documents/repos/tensorflow/
import "C"
import (
	"github.com/pkg/errors"
	"go.viam.com/rdk/config"
)

type MLModel interface {
	// Infer takes an already ordered input tensor as an array,
	// and makes an inference on the model, returning an output tensor map
	Infer(inputTensor interface{}) (config.AttributeMap, error)

	// GetMetadata gets the entire model metadata structure from file
	GetMetadata() (interface{}, error)

	// Close closes the model and interpreter that allows inferences to be made, opens up space in memory.
	// All models must be closed when done using
	Close()
}

// FailedToLoadError is the default error message for when expected resources for inference fail to load
func FailedToLoadError(name string) error {
	return errors.Errorf("failed to load %s", name)
}

// FailedToGetError is the default error message for when expected information will be fetched fails
func FailedToGetError(name string) error {
	return errors.Errorf("failed to get %s", name)
}
