package vision

import (
	"context"

	"go.viam.com/rdk/component/camera"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/utils"
	"go.viam.com/rdk/vision"
)

func (vs *visionService) GetObjectPointClouds(
	ctx context.Context,
	cameraName, segmenterName string,
	params config.AttributeMap,
) ([]*vision.Object, error) {
	cam, err := camera.FromRobot(vs.r, cameraName)
	if err != nil {
		return nil, err
	}
	segmenter, err := vs.segReg.segmenterLookup(segmenterName)
	if err != nil {
		return nil, err
	}
	return segmenter.Segmenter(ctx, cam, params)
}

func (vs *visionService) GetSegmenterNames(ctx context.Context) ([]string, error) {
	return vs.segReg.segmenterNames(), nil
}

func (vs *visionService) GetSegmenterParameters(ctx context.Context, segmenterName string) ([]utils.TypedName, error) {
	segmenter, err := vs.segReg.segmenterLookup(segmenterName)
	if err != nil {
		return nil, err
	}
	return segmenter.Parameters, nil
}
