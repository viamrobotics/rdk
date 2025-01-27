package transformpipeline

import (
	"context"
	"fmt"

	"github.com/invopop/jsonschema"

	"go.viam.com/rdk/components/camera"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
)

// transformType is the list of allowed transforms that can be used in the pipeline.
type transformType string

// the allowed transforms.
const (
	transformTypeUnspecified     = transformType("")
	transformTypeRotate          = transformType("rotate")
	transformTypeResize          = transformType("resize")
	transformTypeCrop            = transformType("crop")
	transformTypeDetections      = transformType("detections")
	transformTypeClassifications = transformType("classifications")
)

// transformRegistration holds pertinent information regarding the available transforms.
type transformRegistration struct {
	name        string
	retType     interface{}
	description string
}

// registeredTransformConfigs is a map of all available transform configs, used for populating fields in the front-end.
var registeredTransformConfigs = map[transformType]*transformRegistration{
	transformTypeRotate: {
		string(transformTypeRotate),
		&rotateConfig{},
		"Rotate the image by 180 degrees. Used when the camera is installed upside down.",
	},
	transformTypeResize: {
		string(transformTypeResize),
		&resizeConfig{},
		"Resizes the image to the specified height and width",
	},
	transformTypeCrop: {
		string(transformTypeCrop),
		&cropConfig{},
		"Crop the image to the specified rectangle in pixels",
	},
	transformTypeDetections: {
		string(transformTypeDetections),
		&detectorConfig{},
		"Overlays object detections on the image. Can use any detector registered in the vision service.",
	},
	transformTypeClassifications: {
		string(transformTypeClassifications),
		&classifierConfig{},
		"Overlays image classifications on the image. Can use any classifier registered in the vision service.",
	},
}

// Transformation states the type of transformation and the attributes that are specific to the given type.
type Transformation struct {
	Type       string             `json:"type"`
	Attributes utils.AttributeMap `json:"attributes"`
}

// JSONSchema defines the schema for each of the possible transforms in the pipeline in a OneOf.
func (Transformation) JSONSchema() *jsonschema.Schema {
	schemas := make([]*jsonschema.Schema, 0, len(registeredTransformConfigs))
	for _, transformReg := range registeredTransformConfigs {
		transformSchema := jsonschema.Reflect(transformReg.retType)
		transformSchema.Title = transformReg.name
		transformSchema.Type = "object"
		transformSchema.Description = transformReg.description
		schemas = append(schemas, transformSchema)
	}
	return &jsonschema.Schema{
		OneOf: schemas,
	}
}

// buildTransform uses the Transformation config to build the desired transform ImageSource.
func buildTransform(
	ctx context.Context,
	r robot.Robot,
	source camera.VideoSource,
	stream camera.ImageType,
	tr Transformation,
) (camera.VideoSource, camera.ImageType, error) {
	switch transformType(tr.Type) {
	case transformTypeUnspecified:
		return source, stream, nil
	case transformTypeRotate:
		return newRotateTransform(ctx, source, stream, tr.Attributes)
	case transformTypeResize:
		return newResizeTransform(ctx, source, stream, tr.Attributes)
	case transformTypeCrop:
		return newCropTransform(ctx, source, stream, tr.Attributes)
	case transformTypeDetections:
		return newDetectionsTransform(ctx, source, r, tr.Attributes)
	case transformTypeClassifications:
		return newClassificationsTransform(ctx, source, r, tr.Attributes)
	default:
		return nil, camera.UnspecifiedStream, fmt.Errorf("do not  know camera transform of type %q", tr.Type)
	}
}

func propsFromVideoSource(ctx context.Context, source camera.VideoSource) (camera.Properties, error) {
	var camProps camera.Properties

	if cameraSrc, ok := source.(camera.Camera); ok {
		props, err := cameraSrc.Properties(ctx)
		if err != nil {
			return camProps, err
		}
		camProps = props
	}
	return camProps, nil
}
