package vision

import (
	"gocv.io/x/gocv"
)

type RotateSource struct {
	Original MatSource
}

func (rs *RotateSource) NextColorDepthPair() (gocv.Mat, DepthMap, error) {
	m, d, err := rs.Original.NextColorDepthPair()
	if err != nil {
		return m, d, err
	}
	gocv.Rotate(m, &m, gocv.Rotate180Clockwise)

	if d.HasData() {
		// TODO(erh): make this faster
		dm := d.ToMat()
		defer dm.Close()
		gocv.Rotate(dm, &dm, gocv.Rotate180Clockwise)
		d = NewDepthMapFromMat(dm)
	}

	return m, d, nil
}

func (rs *RotateSource) Close() {
	rs.Original.Close()
}
