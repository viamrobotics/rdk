package camera

import (
	"encoding/hex"

	"github.com/pkg/errors"

	"go.viam.com/rdk/rimage/transform"
)

// AttrConfig is exported to be used as an attribute map for all camera types.
type AttrConfig struct {
	Color              string                             `json:"color"`
	Depth              string                             `json:"depth"`
	Host               string                             `json:"host"`
	Source             string                             `json:"source"`
	Port               int                                `json:"port"`
	Aligned            bool                               `json:"aligned"`
	Debug              bool                               `json:"debug"`
	Stream             string                             `json:"stream"`
	Num                string                             `json:"num"`
	Args               string                             `json:"args"`
	Width              int                                `json:"width"`
	Height             int                                `json:"height"`
	PlaneSize          int                                `json:"plane_size"`
	SegmentSize        int                                `json:"segment_size"`
	ClusterRadius      float64                            `json:"cluster_radius"`
	Tolerance          float64                            `json:"tolerance"`
	ExcludeColors      []string                           `json:"exclude_color_chans"`
	DetectColorString  string                             `json:"detect_color"`
	Format             string                             `json:"format"`
	Path               string                             `json:"path"`
	PathPattern        string                             `json:"path_pattern"`
	Dump               bool                               `json:"dump"`
	Hide               bool                               `json:"hide"`
	CameraParameters   *transform.PinholeCameraIntrinsics `json:"camera_parameters"`
	IntrinsicExtrinsic interface{}                        `json:"intrinsic_extrinsic"`
	Homography         interface{}                        `json:"homography"`
	Warp               interface{}                        `json:"warp"`
}

// DetectColor transforms the color hexstring into a slice of uint8.
func (ac *AttrConfig) DetectColor() ([]uint8, error) {
	if ac.DetectColorString == "" {
		return []uint8{}, nil
	}
	pound, color := ac.DetectColorString[0], ac.DetectColorString[1:]
	if pound != '#' {
		return nil, errors.Errorf("detect_color is ill-formed, expected #RRGGBB, got %v", ac.DetectColorString)
	}
	slice, err := hex.DecodeString(color)
	if err != nil {
		return nil, err
	}
	if len(slice) != 3 {
		return nil, errors.Errorf("detect_color is ill-formed, expected #RRGGBB, got %v", ac.DetectColorString)
	}
	return slice, nil
}
