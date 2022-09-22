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
	CamIntrinsics     *transform.PinholeCameraIntrinsics `json:"intrinsic_parameters"`
	ScaleEstimatorCfg *ScaleEstimatorConfig              `json:"scale_estimator"`
	CamHeightGround   float64                            `json:"cam_height_ground_m"`
}

// LoadMotionEstimationConfig loads a motion estimation configuration from a json file.
func LoadMotionEstimationConfig(path string) (*MotionEstimationConfig, error) {
	var config MotionEstimationConfig
	configFile, err := os.Open(path) //nolint:gosec
	defer utils.UncheckedErrorFunc(configFile.Close)
	if err != nil {
		return nil, err
	}
	jsonParser := json.NewDecoder(configFile)
	err = jsonParser.Decode(&config)
	if err != nil {
		return nil, err
	}
	return &config, nil
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
func EstimateMotionFrom2Frames(img1, img2 *rimage.Image, cfg *MotionEstimationConfig, logger golog.Logger,
) (*Motion3D, image.Image, error) {
	// Convert both images to gray
	im1 := rimage.MakeGray(img1)
	im2 := rimage.MakeGray(img2)
	sampleMethod := cfg.KeyPointCfg.BRIEFConf.Sampling
	sampleN := cfg.KeyPointCfg.BRIEFConf.N
	samplePatchSize := cfg.KeyPointCfg.BRIEFConf.PatchSize
	samplePoints := keypoints.GenerateSamplePairs(sampleMethod, sampleN, samplePatchSize)
	// compute keypoints
	orb1, kps1, err := keypoints.ComputeORBKeypoints(im1, samplePoints, cfg.KeyPointCfg)
	if err != nil {
		return nil, nil, err
	}
	orb2, kps2, err := keypoints.ComputeORBKeypoints(im2, samplePoints, cfg.KeyPointCfg)
	if err != nil {
		return nil, nil, err
	}
	// match descriptors
	matches := keypoints.MatchDescriptors(orb1, orb2, cfg.MatchingCfg, logger)
	// get 2 sets of matching keypoints
	matchedKps1, matchedKps2, err := keypoints.GetMatchingKeyPoints(matches, kps1, kps2)
	if err != nil {
		return nil, nil, err
	}
	matchedOrbPts1 := keypoints.PlotKeypoints(im1, matchedKps1)
	matchedOrbPts2 := keypoints.PlotKeypoints(im2, matchedKps2)
	matchedLines := keypoints.PlotMatchedLines(matchedOrbPts1, matchedOrbPts2, matchedKps1, matchedKps2, true)
	// get intrinsics matrix
	k := cfg.CamIntrinsics.GetCameraMatrix()

	// Estimate camera pose
	matchedKps1Float := convertImagePointSliceToFloatPointSlice(matchedKps1)
	matchedKps2Float := convertImagePointSliceToFloatPointSlice(matchedKps2)
	pose, err := transform.EstimateNewPose(matchedKps1Float, matchedKps2Float, k)
	if err != nil {
		return nil, matchedLines, err
	}

	// Rescale motion
	estimatedCamHeight, err := EstimateCameraHeight(matchedKps1Float, matchedKps2Float, pose, cfg.ScaleEstimatorCfg, cfg.CamIntrinsics)
	if err != nil {
		return nil, matchedLines, err
	}
	scale := cfg.CamHeightGround / estimatedCamHeight

	var rescaledTranslation mat.Dense
	rescaledTranslation.Scale(scale, pose.Translation)

	return &Motion3D{
		Rotation:    pose.Rotation,
		Translation: &rescaledTranslation,
	}, matchedLines, nil
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
