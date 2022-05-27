package inference

import (
	"bytes"
	"io/ioutil"

	"github.com/pkg/errors"

	"go.viam.com/rdk/ml/inference/tflite"
	metadata "go.viam.com/rdk/ml/inference/tflite_metadata"
)

const tfLiteMetadataName string = "TFLITE_METADATA"

// GetMetadataBytes takes a model path of a tflite file and extracts the metadata buffer from the entire model.
func GetMetadataBytes(modelPath string) ([]byte, error) {
	buf, err := ioutil.ReadFile(modelPath)
	if err != nil {
		return nil, err
	}

	model := tflite.GetRootAsModel(buf, 0)
	metadataLen := model.MetadataLength()
	if metadataLen == 0 {
		return nil, nil
	}

	for i := 0; i < metadataLen; i++ {
		metadata := &tflite.Metadata{}
		success := model.Metadata(metadata, i)
		if !success {
			return nil, errors.New("failed to assign metadata")
		}

		if bytes.Equal([]byte(tfLiteMetadataName), metadata.Name()) {
			metadataBuffer := &tflite.Buffer{}
			success := model.Buffers(metadataBuffer, int(metadata.Buffer()))
			if !success {
				return nil, errors.New("failed to assign metadata buffer")
			}

			bufInBytes := metadataBuffer.DataBytes()
			return bufInBytes, nil
		}
	}

	return nil, nil
}

// getMetadataAsStruct turns the metadata buffer into a readable struct.
func getMetadataAsStruct(metaBytes []byte) *metadata.ModelMetadataT {
	meta := metadata.GetRootAsModelMetadata(metaBytes, 0)
	structMeta := meta.UnPack()
	return structMeta
}
