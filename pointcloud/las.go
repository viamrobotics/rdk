package pointcloud

import (
	"bytes"
	"encoding/binary"
	"image/color"

	"github.com/edaniels/lidario"
	"github.com/golang/geo/r3"
	"go.uber.org/multierr"
	"go.viam.com/utils"
)

const pointValueDataTag = "rc|pv"

// newFromLASFile returns a point cloud from reading a LAS file. If any
// lossiness of points could occur from reading it in, it's reported but is not
// an error.
func newFromLASFile(fn string, cfg TypeConfig) (PointCloud, error) {
	lf, err := lidario.NewLasFile(fn, "r")
	if err != nil {
		return nil, err
	}
	defer utils.UncheckedErrorFunc(lf.Close)

	var hasValue bool
	var valueData []byte
	for _, d := range lf.VlrData {
		if d.Description == pointValueDataTag {
			hasValue = true
			valueData = d.BinaryData
			break
		}
	}

	pc := cfg.NewWithParams(0)

	for i := 0; i < lf.Header.NumberPoints; i++ {
		p, err := lf.LasPoint(i)
		if err != nil {
			return nil, err
		}
		data := p.PointData()

		x, y, z := data.X, data.Y, data.Z
		v := r3.Vector{X: x, Y: y, Z: z}
		var dd Data
		if lf.Header.PointFormatID == 2 && p.RgbData() != nil {
			r := uint8(p.RgbData().Red / 256)
			g := uint8(p.RgbData().Green / 256)
			b := uint8(p.RgbData().Blue / 256)
			dd = NewColoredData(color.NRGBA{r, g, b, 255})
		}

		if hasValue {
			value := int(binary.LittleEndian.Uint64(valueData[i*8 : (i*8)+8]))
			if dd == nil {
				dd = NewBasicData()
			}
			dd.SetValue(value)
		}

		if err := pc.Set(v, dd); err != nil {
			return nil, err
		}
	}
	return pc.FinalizeAfterReading()
}

func writeToLASFile(cloud PointCloud, fn string) (err error) {
	lf, err := lidario.NewLasFile(fn, "w")
	if err != nil {
		return
	}
	defer func() {
		cerr := lf.Close()
		err = multierr.Combine(err, cerr)
	}()

	meta := cloud.MetaData()

	pointFormatID := 0
	if meta.HasColor {
		pointFormatID = 2
	}
	if err = lf.AddHeader(lidario.LasHeader{
		PointFormatID: byte(pointFormatID),
	}); err != nil {
		return
	}

	var pVals []int
	if meta.HasValue {
		pVals = make([]int, 0, cloud.Size())
	}
	var lastErr error
	cloud.Iterate(0, 0, func(pos r3.Vector, d Data) bool {
		var lp lidario.LasPointer
		pr0 := &lidario.PointRecord0{
			// floating point lossiness validated/warned from set/load
			X: pos.X,
			Y: pos.Y,
			Z: pos.Z,
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

		if d != nil {
			pr0.Intensity = d.Intensity()
		}

		if meta.HasColor {
			red, green, blue := 255, 255, 255
			if d != nil && d.HasColor() {
				r, g, b := d.RGB255()
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
		if meta.HasValue {
			if d != nil && d.HasValue() {
				pVals = append(pVals, d.Value())
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
	if meta.HasValue {
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

	//nolint:nakedret
	return
}
