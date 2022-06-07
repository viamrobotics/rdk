// package inference allows users to do inference through tflite (tf, pytorch, etc in the future)
package inference

// #cgo LDFLAGS: -L/Users/alexiswei/Documents/repos/tensorflow/bazel-bin/tensorflow/lite/c
// #cgo CFLAGS: -I/Users/alexiswei/Documents/repos/tensorflow/
import "C"
import (
	"go.viam.com/rdk/config"
)

type ValidModelType string

const (
	Tflite ValidModelType = "tflite"
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
