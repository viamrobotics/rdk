package pc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"path/filepath"

	"github.com/edaniels/golog"
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

	var hasValue bool
	var valueData []byte
	if len(lf.VlrData) != 0 {
		// assumes it is float64s in first index
		hasValue = true
		// TODO(erd): verify this is correct data (tag?)
		valueData = lf.VlrData[0].BinaryData
	}

	pc := NewPointCloud()
	for i := 0; i < lf.Header.NumberPoints; i++ {
		p, err := lf.LasPoint(i)
		if err != nil {
			return nil, err
		}
		data := p.PointData()

		// TODO(erd): truncating out float data, fine?
		x, y, z := int(data.X), int(data.Y), int(data.Z)

		var v float64
		if hasValue {
			bits := binary.LittleEndian.Uint64(valueData[i*8 : (i*8)+8])
			v = math.Float64frombits(bits)
			pc.Set(NewFloatPoint(x, y, z, v))
			continue
		}

		// TODO(erd): color data
		pc.Set(NewPoint(x, y, z))
	}
	return pc, nil
}

func (pc *PointCloud) WriteToFile(fn string) error {
	lf, err := lidario.NewLasFile(fn, "w")
	if err != nil {
		return err
	}
	var successful bool
	defer func() {
		if !successful {
			if err := lf.Close(); err != nil {
				golog.Global.Debug(err)
			}
		}
	}()

	pointFormatID := 0
	if pc.hasColor {
		pointFormatID = 2
	}
	if err := lf.AddHeader(lidario.LasHeader{
		PointFormatID: byte(pointFormatID),
	}); err != nil {
		return err
	}

	var pVals []float64
	if pc.hasValue {
		pVals = make([]float64, 0, pc.Size())
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
		if pc.hasValue {
			if fp, ok := p.(FloatPoint); ok {
				pVals = append(pVals, fp.Value())
			} else {
				pVals = append(pVals, math.NaN())
			}
		}
		if err := lf.AddLasPoint(lp); err != nil {
			lastErr = err
			return false
		}
		return true
	})
	if pc.hasValue {
		var buf bytes.Buffer
		for _, v := range pVals {
			bits := math.Float64bits(v)
			bytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(bytes, bits)
			buf.Write(bytes)
		}
		if err := lf.AddVLR(lidario.VLR{
			UserID:                  "",
			Description:             "point values",
			BinaryData:              buf.Bytes(),
			RecordLengthAfterHeader: buf.Len(),
		}); err != nil {
			return err
		}
	}
	if lastErr != nil {
		return lastErr
	}

	successful = true
	return lf.Close()
}
