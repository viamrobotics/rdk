// Package vision is the service that allows you to access various computer vision algorithms
// (like detection, segmentation, tracking, etc) that usually only require a camera or image input.
package vision

import (
	"context"
	"image"

	"github.com/invopop/jsonschema"
	"github.com/pkg/errors"
	servicepb "go.viam.com/api/service/vision/v1"

	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/utils"
	viz "go.viam.com/rdk/vision"
	"go.viam.com/rdk/vision/classification"
	objdet "go.viam.com/rdk/vision/objectdetection"
)

func init() {
	resource.RegisterSubtype(Subtype, resource.SubtypeRegistration[Service]{
		RPCServiceServerConstructor: NewRPCServiceServer,
		RPCServiceHandler:           servicepb.RegisterVisionServiceHandlerFromEndpoint,
		RPCServiceDesc:              &servicepb.VisionService_ServiceDesc,
		RPCClient:                   NewClientFromConn,
		MaxInstance:                 resource.DefaultMaxInstance,
	})
}

// A Service that implements various computer vision algorithms like detection and segmentation.
type Service interface {
	resource.Resource
	// model parameters
	GetModelParameterSchema(ctx context.Context, modelType VisModelType, extra map[string]interface{}) (*jsonschema.Schema, error)
	// detector methods
	DetectorNames(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddDetector(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error
	RemoveDetector(ctx context.Context, detectorName string, extra map[string]interface{}) error
	DetectionsFromCamera(ctx context.Context, cameraName, detectorName string, extra map[string]interface{}) ([]objdet.Detection, error)
	Detections(ctx context.Context, img image.Image, detectorName string, extra map[string]interface{}) ([]objdet.Detection, error)
	// classifier methods
	ClassifierNames(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddClassifier(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error
	RemoveClassifier(ctx context.Context, classifierName string, extra map[string]interface{}) error
	ClassificationsFromCamera(
		ctx context.Context,
		cameraName, classifierName string,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)
	Classifications(
		ctx context.Context,
		img image.Image,
		classifierName string,
		n int,
		extra map[string]interface{},
	) (classification.Classifications, error)
	// segmenter methods
	SegmenterNames(ctx context.Context, extra map[string]interface{}) ([]string, error)
	AddSegmenter(ctx context.Context, cfg VisModelConfig, extra map[string]interface{}) error
	RemoveSegmenter(ctx context.Context, segmenterName string, extra map[string]interface{}) error
	GetObjectPointClouds(ctx context.Context, cameraName, segmenterName string, extra map[string]interface{}) ([]*viz.Object, error)
}

// SubtypeName is the name of the type of service.
const SubtypeName = resource.SubtypeName("vision")

// Subtype is a constant that identifies the vision service resource subtype.
var Subtype = resource.NewSubtype(
	resource.ResourceNamespaceRDK,
	resource.ResourceTypeService,
	SubtypeName,
)

// Named is a helper for getting the named vision's typed resource name.
// RSDK-347 Implements vision's Named.
func Named(name string) resource.Name {
	return resource.NameFromSubtype(Subtype, name)
}

// FromRobot is a helper for getting the named vision service from the given Robot.
func FromRobot(r robot.Robot, name string) (Service, error) {
	return robot.ResourceFromRobot[Service](r, Named(name))
}

// FindFirstName returns name of first vision service found.
func FindFirstName(r robot.Robot) string {
	for _, val := range robot.NamesBySubtype(r, Subtype) {
		return val
	}
	return ""
}

// FirstFromRobot returns the first vision service on this robot.
func FirstFromRobot(r robot.Robot) (Service, error) {
	vis, err := FirstFromLocalRobot(r)
	if err != nil {
		name := FindFirstName(r)
		return FromRobot(r, name)
	}
	return vis, err
}

// FirstFromLocalRobot returns the first vision service on this main robot.
// This will specifically ignore remote resources.
func FirstFromLocalRobot(r robot.Robot) (Service, error) {
	for _, n := range r.ResourceNames() {
		if n.Subtype == Subtype && !n.ContainsRemoteNames() {
			return FromRobot(r, n.ShortName())
		}
	}
	return nil, errors.New("could not find service")
}

// VisModelType defines what vision models are known by the vision service.
type VisModelType string

// VisModelConfig specifies the name of the detector, the type of detector,
// and the necessary parameters needed to build the detector.
type VisModelConfig struct {
	Name       string             `json:"name"`
	Type       string             `json:"type"`
	Parameters utils.AttributeMap `json:"parameters"`
}

// Config contains a list of the user-provided details necessary to register a new vision service.
type Config struct {
	resource.TriviallyValidateConfig
	ModelRegistry []VisModelConfig `json:"register_models"`
}

// Walk implements the config.Walker interface.
func (conf *Config) Walk(visitor utils.Visitor) (*Config, error) {
	for i, cfg := range conf.ModelRegistry {
		name, err := visitor.Visit(cfg.Name)
		if err != nil {
			return nil, err
		}
		cfg.Name = name.(string)

		typ, err := visitor.Visit(cfg.Type)
		if err != nil {
			return nil, err
		}
		cfg.Type = typ.(string)

		params, err := cfg.Parameters.Walk(visitor)
		if err != nil {
			return nil, err
		}
		cfg.Parameters = params.(utils.AttributeMap)

		conf.ModelRegistry[i] = cfg
	}

	return conf, nil
}
