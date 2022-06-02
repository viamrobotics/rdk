// package inference contains functions to access tflite
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
	Infer(inputTensor interface{}) (config.AttributeMap, error)
	GetInfo() (interface{}, error)
	GetMetadata() (interface{}, error)
	Close()
}
