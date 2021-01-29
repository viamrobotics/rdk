package vision

import (
	"github.com/echolabsinc/robotcore/utils"

	"gocv.io/x/gocv"
)

type RotateMatDepthSource struct {
	Original MatDepthSource
}

func (rmds *RotateMatDepthSource) NextMat() (gocv.Mat, error) {
	rotateSrc := utils.RotateMatSource{rmds.Original}
	return rotateSrc.NextMat()
}

func (rmds *RotateMatDepthSource) NextMatDepthPair() (gocv.Mat, *DepthMap, error) {
	m, d, err := rmds.Original.NextMatDepthPair()
	if err != nil {
		return m, d, err
	}
	gocv.Rotate(m, &m, gocv.Rotate180Clockwise)

	if d != nil && d.HasData() {
		// TODO(erh): make this faster
		dm := d.ToMat()
		defer dm.Close()
		gocv.Rotate(dm, &dm, gocv.Rotate180Clockwise)
		d = NewDepthMapFromMat(dm)
	}

	return m, d, nil
}

func (rmds *RotateMatDepthSource) Close() {
	rmds.Original.Close()
}
