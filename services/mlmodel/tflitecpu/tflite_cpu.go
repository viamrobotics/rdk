//go:build !no_tflite && !no_cgo

// Package tflitecpu runs tflite model files on the host's CPU, as an implementation the ML model service.
package tflitecpu

import (
	"context"
	"math"
	fp "path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.opencensus.io/trace"

	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/ml"
	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/ml/inference/tflite_metadata"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/mlmodel"
)

var sModel = resource.DefaultModelFamily.WithModel("tflite_cpu")

func init() {
	resource.RegisterService(mlmodel.API, sModel, resource.Registration[mlmodel.Service, *TFLiteConfig]{
		Constructor: func(
			ctx context.Context,
			_ resource.Dependencies,
			conf resource.Config,
			logger logging.ZapCompatibleLogger,
		) (mlmodel.Service, error) {
			svcConf, err := resource.NativeConfig[*TFLiteConfig](conf)
			if err != nil {
				return nil, err
			}
			return NewTFLiteCPUModel(ctx, svcConf, conf.ResourceName())
		},
	})
}

// TFLiteConfig contains the parameters specific to a tflite_cpu implementation
// of the MLMS (machine learning model service).
type TFLiteConfig struct {
	resource.TriviallyValidateConfig
	// this should come from the attributes of the tflite_cpu instance of the MLMS
	ModelPath  string `json:"model_path"`
	NumThreads int    `json:"num_threads"`
	LabelPath  string `json:"label_path"`
}

// Model is a struct that implements the TensorflowLite CPU implementation of the MLMS.
// It includes the configured parameters, model struct, and associated metadata.
type Model struct {
	resource.Named
	resource.AlwaysRebuild
	resource.TriviallyCloseable
	conf     TFLiteConfig
	model    *inf.TFLiteStruct
	metadata *mlmodel.MLMetadata
	logger   logging.Logger
}

// NewTFLiteCPUModel is a constructor that builds a tflite cpu implementation of the MLMS.
func NewTFLiteCPUModel(ctx context.Context, params *TFLiteConfig, name resource.Name) (mlmodel.Service, error) {
	_, span := trace.StartSpan(ctx, "service::mlmodel::NewTFLiteCPUModel")
	defer span.End()
	var model *inf.TFLiteStruct
	var loader *inf.TFLiteModelLoader
	var err error
	logger := logging.NewLogger("tflite_cpu")

	addModel := func() (*inf.TFLiteStruct, error) {
		if params == nil {
			return nil, errors.New("could not find parameters")
		}
		if params.NumThreads <= 0 {
			loader, err = inf.NewDefaultTFLiteModelLoader()
		} else {
			loader, err = inf.NewTFLiteModelLoader(params.NumThreads)
		}
		if err != nil {
			return nil, errors.Wrap(err, "could not get loader")
		}

		fullpath, err2 := fp.Abs(params.ModelPath)
		if err2 != nil {
			model, err = loader.Load(params.ModelPath)
		} else {
			model, err = loader.Load(fullpath)
		}

		if err != nil {
			if strings.Contains(err.Error(), "failed to load") {
				if err2 != nil {
					return nil, errors.Wrapf(err, "file not found at %s", params.ModelPath)
				}
				return nil, errors.Wrapf(err, "file not found at %s", fullpath)
			}
			return nil, errors.Wrap(err, "loader could not load model")
		}
		return model, nil
	}
	model, err = addModel()
	if err != nil {
		return nil, errors.Wrapf(err, "could not add model from location %s", params.ModelPath)
	}
	return &Model{Named: name.AsNamed(), conf: *params, model: model, logger: logger}, nil
}

// Infer takes the input map and uses the inference package to
// return the result from the tflite cpu model as a map.
func (m *Model) Infer(ctx context.Context, tensors ml.Tensors) (ml.Tensors, error) {
	_, span := trace.StartSpan(ctx, "service::mlmodel::tflite_cpu::Infer")
	defer span.End()

	outTensors, err := m.model.Infer(tensors)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't infer from model %q", m.Name())
	}
	// Fill in the output map with the names from metadata if u have them
	// if at any point this fails, just use the default name.
	results := ml.Tensors{}
	for defaultName, tensor := range outTensors {
		outName := defaultName
		// tensors are usually added in the same order as the metadata was added. The number
		// at the end of the tensor name (after the colon) is essentially an ordinal.
		parts := strings.Split(defaultName, ":") // number after colon associates it with metadata
		if len(parts) > 1 {
			nameInt, err := strconv.Atoi(parts[len(parts)-1])
			if err == nil && len(m.metadata.Outputs) > nameInt && m.metadata.Outputs[nameInt].Name != "" {
				outName = m.metadata.Outputs[nameInt].Name
			} else {
				outName = strings.Join(parts[0:len(parts)-1], ":") // just use default name, add colons back
			}
		}
		results[outName] = tensor
	}
	return results, nil
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
		blindMD := m.blindFillMetadata()
		m.metadata = &blindMD
		m.logger.Infow("error finding metadata in tflite file", "error", err)
		return blindMD, nil
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
		if i == 0 && m.conf.LabelPath != "" {
			td.Extra = map[string]interface{}{"labels": m.conf.LabelPath}
		}
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

	// Add bounding box info to Extra
	if strings.Contains(inputT.Name, "location") && inputT.Content.ContentProperties.Value != nil {
		if order, ok := inputT.Content.ContentProperties.Value.(*tflite_metadata.BoundingBoxPropertiesT); ok {
			td.Extra = map[string]interface{}{
				"boxOrder": order.Index,
			}
		}
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

func (m *Model) blindFillMetadata() mlmodel.MLMetadata {
	var out mlmodel.MLMetadata
	numIn := m.model.Info.InputTensorCount
	numOut := int(math.Min(float64(m.model.Info.OutputTensorCount), float64(len(m.model.Info.OutputTensorTypes))))
	inputList := make([]mlmodel.TensorInfo, 0, numIn)
	outputList := make([]mlmodel.TensorInfo, 0, numOut)

	// Fill from model info, not metadata
	for i := 0; i < numIn; i++ {
		var td mlmodel.TensorInfo
		switch m.model.Info.InputTensorType {
		case inf.UInt8:
			td.DataType = "uint8"
		case inf.Float32:
			td.DataType = "float32"
		}
		td.Shape = m.model.Info.InputShape
		inputList = append(inputList, td)
	}
	for i := 0; i < numOut; i++ {
		var td mlmodel.TensorInfo
		td.DataType = strings.ToLower(m.model.Info.OutputTensorTypes[i])
		if i == 0 && m.conf.LabelPath != "" {
			td.Extra = map[string]interface{}{"labels": m.conf.LabelPath}
		}
		outputList = append(outputList, td)
	}
	out.Inputs = inputList
	out.Outputs = outputList
	return out
}
