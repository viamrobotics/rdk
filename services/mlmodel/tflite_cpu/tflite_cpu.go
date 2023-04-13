// Package tflitecpu is meant to implement a tfliteCPU in the ML model service
package tflitecpu

import (
	"context"
	fp "path/filepath"
	"strconv"
	"strings"

	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/config"
	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/ml/inference/tflite_metadata"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
)

func init() {
	registry.RegisterService(vision.Subtype, resource.DefaultServiceModel, registry.Service{
		Constructor: func(ctx context.Context, deps registry.Dependencies, config config.Service, logger golog.Logger) (interface{}, error) {
			return newTFLiteCPUModel(ctx, deps, config, logger)
		},
	})
}

// TFLiteConfig contains the parameters specific to a tflite_cpu implementation
// of the MLMS (machine learning model service).
type TFLiteConfig struct {
	// this should come from the attributes of the tflite_cpu instance of the MLMS
	ModelPath  string  `json:"model_path"`
	NumThreads int     `json:"num_threads"`
	LabelPath  *string `json:"label_path"`
}

// Model is a struct that implements the TensorflowLite CPU implementation of the MLMS.
// It includes the configured parameters, model struct, and associated metadata.
type Model struct {
	attrs    TFLiteConfig
	model    *inf.TFLiteStruct
	metadata *mlmodel.MLMetadata
}

// newTFLiteCPUModel is a constructor that builds a tflite cpu implementation of the MLMS.
func newTFLiteCPUModel(ctx context.Context, deps registry.Dependencies, conf config.Service, logger golog.Logger) (*Model, error) { //nolint:unparam
	ctx, span := trace.StartSpan(ctx, "service::mlmodel::NewTFLiteCPUModel")
	defer span.End()

	// Read ML model service parameters into a TFLiteDetectorConfig
	var t TFLiteConfig
	tfParams, err := config.TransformAttributeMapToStruct(&t, conf.Attributes)
	if err != nil {
		return &Model{}, errors.Wrapf(err, "error getting parameters from config")
	}
	params, ok := tfParams.(*TFLiteConfig)
	if !ok {
		err := utils.NewUnexpectedTypeError(params, tfParams)
		return &Model{}, errors.Wrapf(err, "can not register tflite cpu model %s", conf.Name)
	}
	return CreateTFLiteCPUModel(ctx, params)
}

// CreateTFLiteCPUModel is a constructor that builds a tflite cpu implementation of the MLMS.
func CreateTFLiteCPUModel(ctx context.Context, params *TFLiteConfig) (*Model, error) {
	// Given those params, add the model
	model, err := addTFLiteModel(ctx, params.ModelPath, &params.NumThreads)
	if err != nil {
		return &Model{}, errors.Wrapf(err, "could not add model from location %s", params.ModelPath)
	}
	return &Model{attrs: *params, model: model}, nil
}

// Infer takes the input map, finds the image (labeled 'image'), and uses the inference
// package to return the result from the tflite cpu model as a map.
func (m *Model) Infer(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	_, span := trace.StartSpan(ctx, "service::mlmodel::tflite_cpu::Infer")
	defer span.End()

	outMap := make(map[string]interface{})
	doInfer := func(input interface{}) (map[string]interface{}, error) {
		outTensors, err := m.model.Infer(input)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't infer from model")
		}
		// Fill in the output map with the names from metadata if u have them
		// Otherwise, do output1, output2, etc.
		for i := 0; i < len(m.model.Info.OutputTensorTypes); i++ {
			// name := m.metadata.Outputs[i].Name
			// if name == "" {
			if m.metadata != nil {
				outMap[m.metadata.Outputs[i].Name] = outTensors[i]
			} else {
				outMap["output"+strconv.Itoa(i)] = outTensors[i]
			}
		}
		return outMap, nil
	}

	// If there's only one thing in the input map, use it.
	if len(input) == 1 {
		for _, in := range input {
			return doInfer(in)
		}
	}
	// If you have more than 1 thing
	if m.metadata != nil {
		// Use metadata if possible to grab input name
		return doInfer(input[m.metadata.Inputs[0].Name])
	}
	// Try to use "input"
	if in, ok := input["input"]; ok {
		return doInfer(in)
	}

	return nil, errors.New("input map has multiple elements and none are named 'input'")
}

// Metadata reads the metadata from your tflite cpu model into the metadata struct
// that we use for the mlmodel service.
func (m *Model) Metadata(ctx context.Context) (mlmodel.MLMetadata, error) {
	_, span := trace.StartSpan(ctx, "service::mlmodel::tflite_cpu::Metadata")
	defer span.End()

	if m.metadata != nil {
		return *m.metadata, nil
	}

	// model.Metadata() and funnel it into this struct
	md, err := m.model.Metadata()
	if err != nil {
		return mlmodel.MLMetadata{}, err
	}
	out := mlmodel.MLMetadata{}
	out.ModelName = md.Name
	out.ModelDescription = md.Description

	numIn := m.model.Info.InputTensorCount
	numOut := m.model.Info.OutputTensorCount
	inputList := make([]mlmodel.TensorInfo, 0, numIn)
	outputList := make([]mlmodel.TensorInfo, 0, numOut)

	// Fill in input info to the best of our abilities (normally just 1 tensor)
	for i := 0; i < numIn; i++ { // for each input Tensor
		inputT := md.SubgraphMetadata[0].InputTensorMetadata[i]
		td := getTensorInfo(inputT)
		// try to guess model type based on input description
		if strings.Contains(inputT.Description, "detect") {
			out.ModelType = "tflite_detector"
		}
		if strings.Contains(inputT.Description, "classif") {
			out.ModelType = "tflite_classifier"
		}

		switch m.model.Info.InputTensorType { // grab from model info, not metadata
		case inf.UInt8:
			td.DataType = "uint8"
		case inf.Float32:
			td.DataType = "float32"
		default:
			td.DataType = ""
		}
		td.Shape = m.model.Info.InputShape
		inputList = append(inputList, td)
	}

	// Fill in output info to the best of our abilities (can be >1 tensor)
	for i := 0; i < numOut; i++ { // for each output Tensor
		outputT := md.SubgraphMetadata[0].OutputTensorMetadata[i]
		td := getTensorInfo(outputT)
		td.DataType = strings.ToLower(m.model.Info.OutputTensorTypes[i]) // grab from model info, not metadata
		outputList = append(outputList, td)
	}

	out.Inputs = inputList
	out.Outputs = outputList
	m.metadata = &out
	return out, nil
}

// getTensorInfo converts the information from the metadata form to the TensorData struct
// that we use in the mlmodel. This method doesn't populate Extra.
func getTensorInfo(inputT *tflite_metadata.TensorMetadataT) mlmodel.TensorInfo {
	td := mlmodel.TensorInfo{ // Fill in what's easy
		Name:        inputT.Name,
		Description: inputT.Description,
		Extra:       nil,
	}

	// Handle the files
	fileList := make([]mlmodel.File, 0, len(inputT.AssociatedFiles))
	for i := 0; i < len(inputT.AssociatedFiles); i++ {
		outFile := mlmodel.File{
			Name:        inputT.AssociatedFiles[i].Name,
			Description: inputT.AssociatedFiles[i].Description,
		}
		switch inputT.AssociatedFiles[i].Type { //nolint:exhaustive
		case tflite_metadata.AssociatedFileTypeTENSOR_AXIS_LABELS:
			outFile.LabelType = mlmodel.LabelTypeTensorAxis
		case tflite_metadata.AssociatedFileTypeTENSOR_VALUE_LABELS:
			outFile.LabelType = mlmodel.LabelTypeTensorValue
		default:
			outFile.LabelType = mlmodel.LabelTypeUnspecified
		}
		fileList = append(fileList, outFile)
	}
	td.AssociatedFiles = fileList
	return td
}

// addTFLiteModel uses the loader (default or otherwise) from the inference package
// to register a tflite model. Default is chosen if there's no numThreads given.
func addTFLiteModel(ctx context.Context, filepath string, numThreads *int) (*inf.TFLiteStruct, error) {
	_, span := trace.StartSpan(ctx, "service::vision::addTFLiteModel")
	defer span.End()
	var model *inf.TFLiteStruct
	var loader *inf.TFLiteModelLoader
	var err error

	if numThreads == nil {
		loader, err = inf.NewDefaultTFLiteModelLoader()
	} else {
		loader, err = inf.NewTFLiteModelLoader(*numThreads)
	}
	if err != nil {
		return nil, errors.Wrap(err, "could not get loader")
	}

	fullpath, err2 := fp.Abs(filepath)
	if err2 != nil {
		model, err = loader.Load(filepath)
	} else {
		model, err = loader.Load(fullpath)
	}

	if err != nil {
		if strings.Contains(err.Error(), "failed to load") {
			if err2 != nil {
				return nil, errors.Wrapf(err, "file not found at %s", filepath)
			}
			return nil, errors.Wrapf(err, "file not found at %s", fullpath)
		}
		return nil, errors.Wrap(err, "loader could not load model")
	}

	return model, nil
}
