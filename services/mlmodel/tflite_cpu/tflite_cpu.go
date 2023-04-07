// tfliteCPU is a package to implement a tfliteCPU in the ML model service
package tfliteCPU

import (
	"context"
	"go.viam.com/rdk/robot"
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
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
			return newTFLiteCPUModel(ctx, r, config, logger)
		},
	})
}

// TFLiteDetectorConfig contains the parameters specific to a tflite_cpu
// implementation of the MLMS (machine learning model service).
type TFLiteConfig struct {
	// this should come from the attributes part of the tflite_cpu instance of the MLMS
	ModelPath  string  `json:"model_path"`
	NumThreads int     `json:"num_threads"`
	LabelPath  *string `json:"label_path"`
}

// TFLiteCPUModel is a struct that implements the MLMS.
type TFLiteCPUModel struct {
	attrs    TFLiteConfig
	model    *inf.TFLiteStruct
	metadata *mlmodel.MLMetadata
}

// NewTFLiteCPUModel is a constructor that builds a tflite cpu implementation of the MLMS
func newTFLiteCPUModel(ctx context.Context, r robot.Robot, conf config.Service, logger golog.Logger) (*TFLiteCPUModel, error) {
	ctx, span := trace.StartSpan(ctx, "service::mlmodel::NewTFLiteCPUModel")
	defer span.End()

	// Read ML model service parameters into a TFLiteDetectorConfig
	var t TFLiteConfig
	tfParams, err := config.TransformAttributeMapToStruct(&t, conf.Attributes)
	if err != nil {
		return &TFLiteCPUModel{}, errors.New("error getting parameters from config")
	}
	params, ok := tfParams.(*TFLiteConfig)
	if !ok {
		err := utils.NewUnexpectedTypeError(params, tfParams)
		return &TFLiteCPUModel{}, errors.Wrapf(err, "register tflite detector %s", conf.Name)
	}
	return CreateTFLiteCPUModel(ctx, params)
}

// CreateTFLiteCPUModel is a constructor that builds a tflite cpu implementation of the MLMS
func CreateTFLiteCPUModel(ctx context.Context, params *TFLiteConfig) (*TFLiteCPUModel, error) {
	// Given those params, add the model
	model, err := addTFLiteModel(ctx, params.ModelPath, &params.NumThreads)
	if err != nil {
		return &TFLiteCPUModel{}, errors.Wrapf(err, "could not add model from location %s", params.ModelPath)
	}
	return &TFLiteCPUModel{attrs: *params, model: model}, nil
}

// Infer needs to take the input, search for something that looks like an image
// and send that image to the tflite inference service.
// Each new image means a new infer call (for now).
func (m *TFLiteCPUModel) Infer(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	_, span := trace.StartSpan(ctx, "service::mlmodel::tflite_cpu::Infer")
	defer span.End()

	outMap := make(map[string]interface{})

	// Grab the image data from the map
	if imgBytes, ok := input["image"]; ok {
		// Maybe try some other names but for nowww...
		outTensors, err := m.model.Infer(imgBytes)
		if err != nil {
			return nil, errors.Wrap(err, "couldn't infer from model")
		}
		// Fill in the output map with the names from metadata if u have them
		// Otherwise, do output1, output2, etc.
		for i := 0; i < len(m.metadata.Outputs); i++ {
			name := m.metadata.Outputs[i].Name
			if name == "" {
				outMap["output"+strconv.Itoa(i)] = outTensors[i]
			} else {
				outMap[m.metadata.Outputs[i].Name] = outTensors[i]
			}
		}
	} else {
		return map[string]interface{}{}, errors.New("could not find image in input map. Give it the name 'image' duh")
	}

	return outMap, nil
}

// Metadata reads the metadata from your tflite model struct into the metadata struct
// that we use for mlmodel service.
func (m *TFLiteCPUModel) Metadata(ctx context.Context) (mlmodel.MLMetadata, error) {
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

	// Fill in input info to the best of our abilities
	for i := 0; i < numIn; i++ { // //for each input Tensor
		inputT := md.SubgraphMetadata[0].InputTensorMetadata[i]
		// try to guess model type based on description
		if strings.Contains(inputT.Description, "detect") {
			out.ModelType = "tflite_detector"
		}
		if strings.Contains(inputT.Description, "classif") {
			out.ModelType = "tflite_classifier"
		}
		td := getTensorInfo(m, inputT)
		inputList = append(inputList, td)
	}
	// Fill in output info to the best of our abilities
	for i := 0; i < numOut; i++ { // //for each output Tensor
		outputT := md.SubgraphMetadata[0].OutputTensorMetadata[i]
		td := getTensorInfo(m, outputT)
		outputList = append(outputList, td)
	}
	out.Inputs = inputList
	out.Outputs = outputList

	m.metadata = &out
	return out, nil
}

// getTensorInfo converts the information from the metadata form to the TensorData struct
// that we use in the mlmodel. This method doesn't populate Extra or NDim, and only
// populates DataType if metadata reports a range between 0-255.
func getTensorInfo(m *TFLiteCPUModel, inputT *tflite_metadata.TensorMetadataT) mlmodel.TensorInfo {
	td := mlmodel.TensorInfo{ // Fill in what's easy
		Name:        inputT.Name,
		Description: inputT.Description,
		Extra:       nil,
	}

	switch m.model.Info.InputTensorType {
	// grab from model info, not metadata
	case inf.UInt8:
		td.DataType = "uint8"
	case inf.Float32:
		td.DataType = "float32"
	default:
		td.DataType = ""
	}

	// Handle the files
	fileList := make([]mlmodel.File, 0, len(inputT.AssociatedFiles))
	for i := 0; i < len(inputT.AssociatedFiles); i++ {
		outFile := mlmodel.File{
			Name:        inputT.AssociatedFiles[i].Name,
			Description: inputT.AssociatedFiles[i].Description,
		}
		switch inputT.AssociatedFiles[i].Type { // nolint:exhaustivegit
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
