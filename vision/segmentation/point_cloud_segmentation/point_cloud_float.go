package point_cloud_segmentation

import (
	"bytes"
	"encoding/binary"
	"github.com/edaniels/golog"
	"github.com/jblindsay/lidario"
	"math"

	"github.com/golang/geo/r3"
)

type keyFloat r3.Vector

// Structure for point cloud with float64 coordinates
// Adapted from pointcloud/pointcloud.go
type PointCloudFloat struct {
	points     map[keyFloat]PointFloat
	hasColor   bool
	hasValue   bool
	minX, maxX float64
	minY, maxY float64
	minZ, maxZ float64
}

func New() *PointCloudFloat {
	return &PointCloudFloat{
		points: map[keyFloat]PointFloat{},
		minX:   float64(math.MaxInt64),
		minY:   float64(math.MaxInt64),
		minZ:   float64(math.MaxInt64),
		maxX:   float64(math.MaxInt64),
		maxY:   float64(math.MaxInt64),
		maxZ:   float64(math.MaxInt64),
	}
}

func (cloud *PointCloudFloat) Size() int {
	return len(cloud.points)
}

func (cloud *PointCloudFloat) At(x, y, z float64) PointFloat {
	return cloud.points[keyFloat{x, y, z}]
}

func (cloud *PointCloudFloat) Set(p PointFloat) {
	cloud.points[keyFloat(p.Position())] = p
	if ok, _ := IsColored(p); ok {
		cloud.hasColor = true
	}
	if ok, _ := IsValue(p); ok {
		cloud.hasValue = true
	}
	v := p.Position()
	if v.X > cloud.maxX {
		cloud.maxX = v.X
	}
	if v.Y > cloud.maxY {
		cloud.maxY = v.Y
	}
	if v.Z > cloud.maxZ {
		cloud.maxZ = v.Z
	}

	if v.X < cloud.minX {
		cloud.minX = v.X
	}
	if v.Y < cloud.minY {
		cloud.minY = v.Y
	}
	if v.Z < cloud.minZ {
		cloud.minZ = v.Z
	}
}

func (cloud *PointCloudFloat) Unset(x, y, z float64) {
	delete(cloud.points, keyFloat{x, y, z})
}

func (cloud *PointCloudFloat) Iterate(fn func(p PointFloat) bool) {
	for _, p := range cloud.points {
		if cont := fn(p); !cont {
			return
		}
	}
}

const pointValueDataTag = "rc|pv"
func (pc *PointCloudFloat) WriteToFile(fn string) error {
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

	var pVals []int
	if pc.hasValue {
		pVals = make([]int, 0, pc.Size())
	}
	var lastErr error
	pc.Iterate(func(p PointFloat) bool {
		pos := p.Position()
		var lp lidario.LasPointer
		pr0 := &lidario.PointRecord0{
			X:         float64(pos.X), // TODO(erd): may be lossy
			Y:         float64(pos.Y), // TODO(erd): may be lossy
			Z:         float64(pos.Z), // TODO(erd): may be lossy
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
			if ok, cp := IsColored(p); ok {
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
			if ok, fp := IsValue(p); ok {
				pVals = append(pVals, fp.Value())
			} else {
				pVals = append(pVals, 0)
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
			bytes := make([]byte, 8)
			binary.LittleEndian.PutUint64(bytes, uint64(v))
			buf.Write(bytes)
		}
		if err := lf.AddVLR(lidario.VLR{
			UserID:                  "",
			Description:             pointValueDataTag,
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