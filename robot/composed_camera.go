package robot

import (
	"fmt"

	"github.com/edaniels/gostream"

	"go.viam.com/robotcore/rimage"
)

func newDepthComposed(r *Robot, config Component) (gostream.ImageSource, error) {
	color := r.CameraByName(config.Attributes["color"])
	if color == nil {
		return nil, fmt.Errorf("cannot find color camera (%s)", config.Attributes["color"])
	}
	depth := r.CameraByName(config.Attributes["depth"])
	if depth == nil {
		return nil, fmt.Errorf("cannot find depth camera (%s)", config.Attributes["depth"])
	}

	return rimage.NewDepthComposed(color, depth)
}
