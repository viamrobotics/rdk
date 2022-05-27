package main

import "C"
import (
	"bytes"
	"io/ioutil"
	"log"

	"github.com/pkg/errors"

	"go.viam.com/rdk/protoutils"
	metadata "go.viam.com/rdk/testing/checkApi/tflite_metadata"
	"go.viam.com/rdk/testing/tflite"
	"google.golang.org/protobuf/types/known/structpb"
)

const tfLiteMetadataName string = "TFLITE_METADATA"

func main() {
	// currentDirectory, err := os.Getwd()
	// if err != nil {
	// 	log.Println(err)
	// }
	var modelPath = "/Users/alexiswei/Documents/repos/rdk/testing/model_with_metadata.tflite"
	metadataBuffer, err := GetMetadataBuffer(modelPath)
	if err != nil {
		log.Println(err)
	}

	if metadataBuffer == nil {
		return
	}
	metadata := ReadMetadataBytes(metadataBuffer)

	metaProto, err := StructToStructPb(metadata)
	if err != nil {
		log.Println(err)
	}
	log.Println(metaProto)
	// fmt.Println(metadataBuffer)
}

// GetMetadata takes a model path of a tflite file and extracts the metadata buffer from the entire model
func GetMetadataBuffer(modelPath string) ([]byte, error) {
	buf, err := ioutil.ReadFile(modelPath)
	if err != nil {
		// fmt.Println("File reading error", err)
		return nil, err
	}

	model := tflite.GetRootAsModel(buf, 0)
	metadataLength := model.MetadataLength()
	if metadataLength == 0 {
		// fmt.Println("no metadata")
		return nil, nil
	}

	for i := 0; i < metadataLength; i++ {
		// check name of the metadata element
		metadata := &tflite.Metadata{}
		success := model.Metadata(metadata, i)
		if !success {
			return nil, errors.New("failed to assign metadata")
		}

		name := metadata.Name()
		// fmt.Println(string(name))
		if bytes.Equal([]byte(tfLiteMetadataName), name) {
			metadataBuffer := &tflite.Buffer{}
			success := model.Buffers(metadataBuffer, int(metadata.Buffer()))
			if !success {
				return nil, errors.New("failed to assign metadata buffer")
			}

			bufInBytes := metadataBuffer.DataBytes()
			// fmt.Println(bufInBytes)

			return bufInBytes, nil
		}
	}

	return nil, nil
}

var dat map[string]map[string]interface{}

func ReadMetadataBytes(metaBuf []byte) *metadata.ModelMetadataT {
	// fmt.Println("hello")
	meta := metadata.GetRootAsModelMetadata(metaBuf, 0)
	structMeta := meta.UnPack()
	// fmt.Println(string(meta.Name()))
	// fmt.Println(structMeta.Name)
	// Output1 := structMeta.SubgraphMetadata[0].OutputTensorMetadata[0].Name
	// Output2 := structMeta.SubgraphMetadata[0].OutputTensorMetadata[1].Name
	// // fmt.Println(Output1)
	// fmt.Println(Output2)
	// fmt.Println(string(meta.Table().Bytes))

	return structMeta
}

// StructToStructPb converts an arbitrary Go struct to a *structpb.Struct. Only exported fields are included in the
// returned proto.
func StructToStructPb(i interface{}) (*structpb.Struct, error) {
	encoded, err := protoutils.InterfaceToMap(i)
	if err != nil {
		return nil, errors.Errorf("unable to convert interface %v to a form acceptable to structpb.NewStruct: %v", i, err)
	}
	ret, err := structpb.NewStruct(encoded)
	if err != nil {
		return nil, errors.Errorf("unable to construct structpb.Struct from map %v: %v", encoded, err)
	}
	return ret, nil
}
