package camera

import (
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
	DetectColor        []uint8                            `json:"detect_color"`
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
