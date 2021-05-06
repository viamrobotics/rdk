package pointcloud

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image/color"
	"io"
	"path/filepath"

	"github.com/edaniels/golog"
	"github.com/jblindsay/lidario"

	"go.uber.org/multierr"
)

func NewFromFile(fn string, logger golog.Logger) (PointCloud, error) {
	switch filepath.Ext(fn) {
	case ".las":
		return NewFromLASFile(fn, logger)
	default:
		return nil, fmt.Errorf("do not know how to read file %q", fn)
	}
}

const pointValueDataTag = "rc|pv"

func NewFromLASFile(fn string, logger golog.Logger) (PointCloud, error) {
	lf, err := lidario.NewLasFile(fn, "r")
	if err != nil {
		return nil, err
	}
	defer lf.Close()

	var hasValue bool
	var valueData []byte
	for _, d := range lf.VlrData {
		if d.Description == pointValueDataTag {
			hasValue = true
			valueData = d.BinaryData
			break
		}
	}

	pc := New()
	for i := 0; i < lf.Header.NumberPoints; i++ {
		p, err := lf.LasPoint(i)
		if err != nil {
			return nil, err
		}
		data := p.PointData()

		x, y, z := data.X, data.Y, data.Z
		if x < minPreciseFloat64 || x > maxPreciseFloat64 ||
			y < minPreciseFloat64 || y > maxPreciseFloat64 ||
			z < minPreciseFloat64 || z > maxPreciseFloat64 {
			logger.Warnf("potential floating point lossiness for LAS point",
				"point", data, "range", fmt.Sprintf("[%d,%d]", minPreciseFloat64, maxPreciseFloat64))
		}
		pToSet := NewBasicPoint(x, y, z)

		if lf.Header.PointFormatID == 2 && p.RgbData() != nil {
			r := uint8(p.RgbData().Red / 256)
			g := uint8(p.RgbData().Green / 256)
			b := uint8(p.RgbData().Blue / 256)
			pToSet.SetColor(color.NRGBA{r, g, b, 255})
		}

		if hasValue {
			value := int(binary.LittleEndian.Uint64(valueData[i*8 : (i*8)+8]))
			pToSet.SetValue(value)
		}

		if err := pc.Set(pToSet); err != nil {
			return nil, err
		}
	}
	return pc, nil
}

func (pc *basicPointCloud) WriteToFile(fn string) (err error) {
	lf, err := lidario.NewLasFile(fn, "w")
	if err != nil {
		return
	}
	defer func() {
		cerr := lf.Close()
		err = multierr.Combine(err, cerr)
	}()

	pointFormatID := 0
	if pc.hasColor {
		pointFormatID = 2
	}
	if err = lf.AddHeader(lidario.LasHeader{
		PointFormatID: byte(pointFormatID),
	}); err != nil {
		return
	}

	var pVals []int
	if pc.hasValue {
		pVals = make([]int, 0, pc.Size())
	}
	var lastErr error
	pc.Iterate(func(p Point) bool {
		pos := p.Position()
		var lp lidario.LasPointer
		pr0 := &lidario.PointRecord0{
			// floating point losiness validated/warned from set/load
			X:         pos.X,
			Y:         pos.Y,
			Z:         pos.Z,
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
			if p.HasColor() {
				r, g, b := p.RGB255()
				red, green, blue = int(r), int(g), int(b)
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
		if pc.hasValue {
			if p.HasValue() {
				pVals = append(pVals, p.Value())
			} else {
				pVals = append(pVals, 0)
			}
		}
		if lerr := lf.AddLasPoint(lp); lerr != nil {
			lastErr = lerr
			return false
		}
		return true
	})
	if pc.hasValue {
		var buf bytes.Buffer
		for _, v := range pVals {
			bytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(bytes, uint64(v))
			buf.Write(bytes)
		}
		if err = lf.AddVLR(lidario.VLR{
			UserID:                  "",
			Description:             pointValueDataTag,
			BinaryData:              buf.Bytes(),
			RecordLengthAfterHeader: buf.Len(),
		}); err != nil {
			return
		}
	}
	if lastErr != nil {
		err = lastErr
		return
	}

	return
}

func _colorToPCDInt(pt Point) int {
	r, g, b := pt.RGB255()
	x := 0

	x = x | (int(r) << 16)
	x = x | (int(g) << 8)
	x = x | (int(b) << 0)
	return x
}

func (pc *basicPointCloud) ToPCD(out io.Writer) error {
	var err error

	_, err = fmt.Fprintf(out, "VERSION .7\n"+
		"FIELDS x y z rgb\n"+
		"SIZE 4 4 4 4\n"+
		"TYPE F F F I\n"+
		"COUNT 1 1 1 1\n"+
		"WIDTH %d\n"+
		"HEIGHT %d\n"+
		"VIEWPOINT 0 0 0 1 0 0 0\n"+
		"POINTS %d\n"+
		"DATA ascii\n",
		pc.Size(),
		1,
		pc.Size(),
	)

	if err != nil {
		return err
	}

	pc.Iterate(func(pt Point) bool {
		// Our pointclouds are in mm, PCD files expect meters
		position := pt.Position()
		width := position.X / 1000.
		height := -position.Y / 1000.
		depth := -position.Z / 1000.

		_, err = fmt.Fprintf(out, "%f %f %f %d\n",
			width,
			height,
			depth,
			_colorToPCDInt(pt))
		return err == nil
	})

	if err != nil {
		return err
	}
	return nil
}
