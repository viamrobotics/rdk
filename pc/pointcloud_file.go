package pc

import (
	"fmt"
	"path/filepath"

	"github.com/jblindsay/lidario"
)

func NewPointCloudFromFile(fn string) (*PointCloud, error) {
	switch filepath.Ext(fn) {
	case ".las":
		return newPointCloudFromLASFile(fn)
	default:
		return nil, fmt.Errorf("do not know how to read file %q", fn)
	}
}

func newPointCloudFromLASFile(fn string) (*PointCloud, error) {
	lf, err := lidario.NewLasFile(fn, "r")
	if err != nil {
		return nil, err
	}
	defer lf.Close()

	pc := NewPointCloud()
	for i := 0; i < lf.Header.NumberPoints; i++ {
		p, err := lf.LasPoint(i)
		if err != nil {
			return nil, err
		}
		data := p.PointData()

		// TODO(erd): losing float data
		// TODO(erd): color data
		pc.Set(NewPoint(
			int(data.X),
			int(data.Y),
			int(data.Z)))
	}
	return pc, nil
}

func (pc *PointCloud) WriteToFile(fn string) error {
	lf, err := lidario.NewLasFile(fn, "w")
	if err != nil {
		return err
	}

	pointFormatID := 0
	if pc.hasColor {
		pointFormatID = 2
	}
	if err := lf.AddHeader(lidario.LasHeader{
		PointFormatID: byte(pointFormatID),
	}); err != nil {
		return err
	}

	var lastErr error
	pc.Iterate(func(p Point) bool {
		pos := p.Position()
		var lp lidario.LasPointer
		pr0 := &lidario.PointRecord0{
			X:         float64(pos.X),
			Y:         float64(pos.Y),
			Z:         float64(pos.Z),
			Intensity: 0,
			BitField: lidario.PointBitField{
				Value: (1) | (1 << 3) | (0 << 6) | (0 << 7),
			},
			ClassBitField: lidario.ClassificationBitField{
				Value: 0,
			},
			ScanAngle:     0,
			UserData:      0,
			PointSourceID: 1,
		}
		lp = pr0
		if pc.hasColor {
			red, green, blue := 255, 255, 255
			if cp, ok := p.(ColoredPoint); ok {
				c := cp.Color()
				red, green, blue = int(c.R), int(c.G), int(c.B)
			}
			lp = &lidario.PointRecord2{
				PointRecord0: pr0,
				RGB: &lidario.RgbData{
					Red:   uint16(red * 256),
					Green: uint16(green * 256),
					Blue:  uint16(blue * 256),
				},
			}
		}
		if err := lf.AddLasPoint(lp); err != nil {
			lastErr = err
			return false
		}
		return true
	})
	if lastErr != nil {
		if err := lf.Close(); err != nil {
			return err
		}
		return lastErr
	}

	return lf.Close()
}
