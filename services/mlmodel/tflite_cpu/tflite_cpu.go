package tflite_cpu

import (
	"context"
	"github.com/edaniels/golog"
	"github.com/pkg/errors"
	"go.opencensus.io/trace"
	"go.viam.com/rdk/config"
	inf "go.viam.com/rdk/ml/inference"
	"go.viam.com/rdk/ml/inference/tflite_metadata"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/services/mlmodel"
	"go.viam.com/rdk/services/vision"
	"go.viam.com/rdk/utils"
	fp "path/filepath"
	"strconv"
	"strings"
)

func init() {
	registry.RegisterService(vision.Subtype, resource.DefaultServiceModel, registry.Service{
		RobotConstructor: func(ctx context.Context, r robot.Robot, config config.Service, logger golog.Logger) (interface{}, error) {
			return NewTFLiteCPUModel(ctx, r, config, logger)
		},
	})
}

type TFLiteDetectorConfig struct {
	// this should come from the attributes part of the tflite_cpu instance of the MLMS
	ModelPath  string  `json:"model_path"`
	NumThreads int     `json:"num_threads"`
	LabelPath  *string `json:"label_path"`
}
type tfliteCPUModel struct {
	attrs    TFLiteDetectorConfig
	model    *inf.TFLiteStruct
	metadata *mlmodel.MLMetadata
}

func NewTFLiteCPUModel(ctx context.Context, robot robot.Robot, conf config.Service, logger golog.Logger) (*tfliteCPUModel, error) {
	ctx, span := trace.StartSpan(ctx, "service::mlmodel::NewTFLiteCPUModel")
	defer span.End()

	// Read ML model service parameters into a TFLiteDetectorConfig
	var t TFLiteDetectorConfig
	tfParams, err := config.TransformAttributeMapToStruct(&t, conf.Attributes)
	if err != nil {
		return &tfliteCPUModel{}, errors.New("error getting parameters from config")
	}
	params, ok := tfParams.(*TFLiteDetectorConfig)
	if !ok {
		err := utils.NewUnexpectedTypeError(params, tfParams)
		return &tfliteCPUModel{}, errors.Wrapf(err, "register tflite detector %s", conf.Name)
	}

	// Given those params, add the model
	model, err := addTFLiteModel(ctx, params.ModelPath, &params.NumThreads)
	if err != nil {
		return &tfliteCPUModel{}, errors.Wrapf(err, "could not add model from location %s", params.ModelPath)
	}
	return &tfliteCPUModel{attrs: *params, model: model}, nil
}

// Infer needs to take the input, search for something that looks like an image
// and send that image to the tflite inference service.
// Each new image means a new infer call (for now).
func (m *tfliteCPUModel) Infer(ctx context.Context, input map[string]interface{}) (map[string]interface{}, error) {
	_, span := trace.StartSpan(ctx, "service::mlmodel::tflite_cpu::Infer")
	defer span.End()

	outMap := make(map[string]interface{})

	// Grab the image data from the map
	if imgBytes, ok := input["image"]; !ok {
		return map[string]interface{}{}, errors.New("could not find image in input map. Give it the name 'image' duh")
	} else {
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
	}
	return outMap, nil
}

// Metadata reads the metadata from your tflite model struct into the metadata struct
// that we use for mlmodel service.
func (m *tfliteCPUModel) Metadata(ctx context.Context) (mlmodel.MLMetadata, error) {
	// model.Metadata() and funnel it into this struct
	md, err := m.model.Metadata()
	if err != nil {
		return mlmodel.MLMetadata{}, err
	}
	out := mlmodel.MLMetadata{}
	out.ModelName = md.Name
	out.ModelDescription = md.Description
	out.ModelType = "tflite_cpu" //for now cuz idk

	numIn := len(md.SubgraphMetadata[0].InputTensorMetadata)
	numOut := len(md.SubgraphMetadata[0].OutputTensorMetadata)
	inputList := make([]mlmodel.TensorInfo, 0, numIn)
	outputList := make([]mlmodel.TensorInfo, 0, numOut)

	// Fill in input info to the best of our abilities
	for i := 0; i < numIn; i++ { // //for each input Tensor
		inputT := md.SubgraphMetadata[0].InputTensorMetadata[i]
		td := getTensorInfo(inputT)
		inputList = append(inputList, td)
	}
	// Fill in output info to the best of our abilities
	for i := 0; i < numOut; i++ { // //for each output Tensor
		outputT := md.SubgraphMetadata[0].OutputTensorMetadata[i]
		td := getTensorInfo(outputT)
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
func getTensorInfo(inputT *tflite_metadata.TensorMetadataT) mlmodel.TensorInfo {
	td := mlmodel.TensorInfo{ // Fill in what's easy
		Name:        inputT.Name,
		Description: inputT.Description,
		Extra:       nil,
	}
	if inputT.Stats.Max[0] == 255 { // Make a guess at this only if we see 255
		td.DataType = "uint8"
	}
	// Handle the files
	fileList := make([]mlmodel.File, 0, len(inputT.AssociatedFiles))
	for i := 0; i <= len(inputT.AssociatedFiles); i++ {
		outFile := mlmodel.File{
			Name:        inputT.AssociatedFiles[i].Name,
			Description: inputT.AssociatedFiles[i].Description,
		}
		switch inputT.AssociatedFiles[i].Type {
		case 2: // TENSOR_AXIS_LABELS
			outFile.LabelType = mlmodel.LabelTypeTensorAxis
		case 3: // TENSOR_VALUE_LABELS
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
