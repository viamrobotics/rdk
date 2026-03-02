// Package visualize provides plotting/visualization utilities for pointcloud data.
// It is separated from the main pointcloud package to avoid pulling heavy plotting
// dependencies (liberation fonts, gonum/plot, go-pdf, go-hep) into binaries that
// use pointcloud but don't need visualization.
package visualize

import (
	"bytes"
	"fmt"
	"image"

	"go-hep.org/x/hep/hbook"
	"go-hep.org/x/hep/hplot"
	vecg "gonum.org/v1/plot/vg"

	pc "go.viam.com/rdk/pointcloud"
)

// VoxelHistogram creates useful plots for determining the parameters of the voxel grid when calibrating a new sensor.
// Histograms of the number of points in each voxel, the weights of each voxel, and the plane residuals.
func VoxelHistogram(vg *pc.VoxelGrid, w, h int, name string) (image.Image, error) {
	var hist *hbook.H1D
	p := hplot.New()
	switch name {
	case "points":
		p.Title.Text = "Points in Voxel"
		p.X.Label.Text = "Pts in Voxel"
		p.Y.Label.Text = "NVoxels"
		hist = hbook.NewH1D(25, 0, +25)
		for _, vox := range vg.Voxels {
			variable := float64(len(vox.Points))
			hist.Fill(variable, 1)
		}
	case "weights":
		hist = hbook.NewH1D(40, 0, +1)
		p.Title.Text = "Weights of Voxel"
		p.X.Label.Text = "Voxel Weight"
		p.Y.Label.Text = "N Vox"
		for _, vox := range vg.Voxels {
			variable := -9.0
			if len(vox.Points) > 5 {
				vox.Center = pc.GetVoxelCenter(vox.Positions())
				vox.Normal = pc.EstimatePlaneNormalFromPoints(vox.Positions())
				vox.Offset = pc.GetOffset(vox.Center, vox.Normal)
				vox.Residual = pc.GetResidual(vox.Positions(), vox.GetPlane())
				variable = pc.GetWeight(vox.Positions(), vg.Lambda(), vox.Residual)
			}
			hist.Fill(variable, 1)
		}
	case "residuals":
		hist = hbook.NewH1D(65, 0, +6.5)
		p.Title.Text = "Residual of Voxel"
		p.X.Label.Text = "Voxel Residuals"
		p.Y.Label.Text = "N Voxels"
		for _, vox := range vg.Voxels {
			variable := -999.
			if len(vox.Points) > 5 {
				vox.Center = pc.GetVoxelCenter(vox.Positions())
				vox.Normal = pc.EstimatePlaneNormalFromPoints(vox.Positions())
				vox.Offset = pc.GetOffset(vox.Center, vox.Normal)
				vox.Residual = pc.GetResidual(vox.Positions(), vox.GetPlane())
				variable = vox.Residual
			}
			hist.Fill(variable, 1)
		}
	default:
		return nil, fmt.Errorf("%s not a plottable variable", name)
	}

	// Create a histogram of our values
	hp := hplot.NewH1D(hist)
	hp.Infos.Style = hplot.HInfoSummary
	p.Add(hp)

	width, err := vecg.ParseLength(fmt.Sprintf("%dpt", w))
	if err != nil {
		return nil, err
	}
	height, err := vecg.ParseLength(fmt.Sprintf("%dpt", h))
	if err != nil {
		return nil, err
	}
	imgByte, err := hplot.Show(p, width, height, "png")
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(imgByte))
	if err != nil {
		return nil, err
	}
	return img, nil
}
