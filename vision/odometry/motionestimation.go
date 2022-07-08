package odometry

import (
	"encoding/json"
	"image"
	"os"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r2"
	"go.viam.com/utils"
	"gonum.org/v1/gonum/mat"

	"go.viam.com/rdk/rimage"
	"go.viam.com/rdk/rimage/transform"
	"go.viam.com/rdk/vision/keypoints"
)

// MotionEstimationConfig contains the parameters needed for motion estimation between two video frames.
type MotionEstimationConfig struct {
	KeyPointCfg       *keypoints.ORBConfig               `json:"kps"`
	MatchingCfg       *keypoints.MatchingConfig          `json:"matching"`
	CamIntrinsics     *transform.PinholeCameraIntrinsics `json:"cam_intrinsics"`
	ScaleEstimatorCfg *ScaleEstimatorConfig              `json:"scale_estimator"`
	CamHeightGround   float64                            `json:"cam_height_ground"`
}

// LoadMotionEstimationConfig loads a motion estimation configuration from a json file.
func LoadMotionEstimationConfig(path string) *MotionEstimationConfig {
	var config MotionEstimationConfig
	configFile, err := os.Open(path) // nolint:gosec
	defer utils.UncheckedErrorFunc(configFile.Close)
	if err != nil {
		return nil
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		return nil
	}
	return &config
}

// Motion3D contains the estimated 3D rotation and translation from 2 frames.
type Motion3D struct {
	Rotation    *mat.Dense
	Translation *mat.Dense
}

// NewMotion3DFromRotationTranslation returns a new pointer to Motion3D from a rotation and a translation matrix.
func NewMotion3DFromRotationTranslation(rotation, translation *mat.Dense) *Motion3D {
	return &Motion3D{
		Rotation:    rotation,
		Translation: translation,
	}
}

// EstimateMotionFrom2Frames estimates the 3D motion of the camera between frame img1 and frame img2.
func EstimateMotionFrom2Frames(img1, img2 *rimage.Image, cfg *MotionEstimationConfig, logger golog.Logger, display bool,
) (*Motion3D, error) {
	var err error
	tempDir, err := os.MkdirTemp("", "motion_keypoint_matching")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)
	// Convert both images to gray
	im1 := rimage.MakeGray(img1)
	im2 := rimage.MakeGray(img2)
	// compute keypoints
	orb1, kps1, err := keypoints.ComputeORBKeypoints(im1, cfg.KeyPointCfg)
	if err != nil {
		return nil, err
	}
	if display {
		logger.Infof("writing motion keypoint files to %s", tempDir)
		err = keypoints.PlotKeypoints(im1, kps1, tempDir+"/img1_orb_points.png")
		if err != nil {
			logger.Warnf("%s/img1_orb_points.png could not be saved", tempDir)
		}
	}
	orb2, kps2, err := keypoints.ComputeORBKeypoints(im2, cfg.KeyPointCfg)
	if err != nil {
		return nil, err
	}
	if display {
		err = keypoints.PlotKeypoints(im2, kps2, tempDir+"/img2_orb_points.png")
		if err != nil {
			logger.Warnf("%s/img2_orb_points.png could not be saved", tempDir)
		}
	}
	// match keypoints
	matches := keypoints.MatchKeypoints(orb1, orb2, cfg.MatchingCfg, logger)
	// get 2 sets of matching keypoints
	matchedKps1, matchedKps2, err := keypoints.GetMatchingKeyPoints(matches, kps1, kps2)
	if display {
		err = keypoints.PlotKeypoints(im1, matchedKps1, tempDir+"/img1.png")
		if err != nil {
			logger.Warnf("%s/img1.png could not be saved", tempDir)
		}
		err = keypoints.PlotKeypoints(im2, matchedKps2, tempDir+"/img2.png")
		if err != nil {
			logger.Warnf("%s/img2.png could not be saved", tempDir)
		}
	}
	if err != nil {
		return nil, err
	}
	// get intrinsics matrix
	k := cfg.CamIntrinsics.GetCameraMatrix()

	// Estimate camera pose
	matchedKps1Float := convertImagePointSliceToFloatPointSlice(matchedKps1)
	matchedKps2Float := convertImagePointSliceToFloatPointSlice(matchedKps2)
	pose, err := transform.EstimateNewPose(matchedKps1Float, matchedKps2Float, k)
	if err != nil {
		return nil, err
	}

	// Rescale motion
	estimatedCamHeight, err := EstimateCameraHeight(matchedKps1Float, matchedKps2Float, pose, cfg.ScaleEstimatorCfg, cfg.CamIntrinsics)
	if err != nil {
		return nil, err
	}
	scale := cfg.CamHeightGround / estimatedCamHeight

	var rescaledTranslation mat.Dense
	rescaledTranslation.Scale(scale, pose.Translation)

	return &Motion3D{
		Rotation:    pose.Rotation,
		Translation: &rescaledTranslation,
	}, nil
}

// convertImagePointSliceToFloatPointSlice is a helper to convert slice of image.Point to a slice of r2.Point.
func convertImagePointSliceToFloatPointSlice(pts []image.Point) []r2.Point {
	ptsOut := make([]r2.Point, len(pts))
	for i, pt := range pts {
		ptsOut[i] = r2.Point{
			X: float64(pt.X),
			Y: float64(pt.Y),
		}
	}
	return ptsOut
}
