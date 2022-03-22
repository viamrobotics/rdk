// Package vectornav implement vectornav imu
package vectornav

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	"github.com/pkg/errors"
	rdkutils "go.viam.com/utils"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	rutils "go.viam.com/rdk/utils"
)

const model = "vectornav"

func init() {
	registry.RegisterComponent(imu.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewVectorNav(ctx, r, config, logger)
		},
	})
}

type vectornav struct {
	angularVelocity spatialmath.AngularVelocity
	acceleration    r3.Vector
	magnetometer    r3.Vector
	dV              r3.Vector
	dTheta          r3.Vector
	dt              float32
	orientation     spatialmath.EulerAngles

	mu      sync.Mutex
	spiMu   sync.Mutex
	polling int

	cancelFunc              func()
	activeBackgroundWorkers sync.WaitGroup
	bus                     board.SPI
	cs                      string
	speed                   int
	logger                  golog.Logger
	busClosed               bool

	bdVX float64
	bdVY float64
	bdVZ float64
}

const (
	vectorNavSPIRead  uint = 1
	vectorNavSPIWrite uint = 2
	vectorNavSPITare  uint = 5
)

type vectornavRegister uint

const (
	modelNumber                  vectornavRegister = 1
	serialNumber                 vectornavRegister = 3
	firmwareVersion              vectornavRegister = 4
	deltaVDeltaTheta             vectornavRegister = 80
	deltaVDeltaThetaConfig       vectornavRegister = 82
	yawPitchRollMagAccGyro       vectornavRegister = 27
	acceleration                 vectornavRegister = 18
	referenceVectorConfiguration vectornavRegister = 83
	magAccRefVectors             vectornavRegister = 21
	accCompensationConfiguration vectornavRegister = 25
	vpeAccTunning                vectornavRegister = 38
)

// NewVectorNav connect and set up a vectornav IMU over SPI.
// Will also compensate for acceleration and delta velocity bias over one second so be sure the IMU is still when calling this function.
func NewVectorNav(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (imu.IMU, error) {
	b, err := board.FromRobot(r, config.Attributes.String("board"))
	if err != nil {
		return nil, errors.Errorf("vectornav init failed couldn't find board %q", config.Attributes.String("board"))
	}
	spiName := config.Attributes.String("spi")
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, errors.Errorf("vectornav: board %q is not local", config.Attributes.String("board"))
	}
	spiBus, ok := localB.SPIByName(spiName)
	if !ok {
		return nil, errors.Errorf("vectornav: couldn't get spi bus %q", spiName)
	}
	cs := config.Attributes.String("cs_pin")
	if cs == "" {
		return nil, errors.New("vectornav: need chip select pin")
	}
	speed := config.Attributes.Int("speed", 8000000)
	pfreq := config.Attributes.Int("polling_freq", 0)
	v := &vectornav{
		bus:       spiBus,
		logger:    logger,
		cs:        cs,
		speed:     speed,
		busClosed: false,
		polling:   pfreq,
	}
	mdl, err := v.readRegisterSPI(ctx, modelNumber, 24)
	if err != nil {
		return nil, err
	}
	sn, err := v.readRegisterSPI(ctx, serialNumber, 4)
	if err != nil {
		return nil, err
	}
	fwver, err := v.readRegisterSPI(ctx, firmwareVersion, 4)
	if err != nil {
		return nil, err
	}
	logger.Debugf("model detected %s sn %d %d.%d.%d.%d", string(mdl), binary.LittleEndian.Uint32(sn), fwver[0], fwver[1], fwver[2], fwver[3])

	// set imu location to New York for the WGM model
	refvec := []byte{1, 1, 0, 0}
	refvec = append(refvec, rutils.BytesFromUint32LE(1000)...)
	refvec = append(refvec, rutils.BytesFromFloat32LE(2010.0)...)
	refvec = append(refvec, []byte{0, 0, 0, 0}...)
	refvec = append(refvec, rutils.BytesFromFloat64LE(40.730610)...)
	refvec = append(refvec, rutils.BytesFromFloat64LE(-73.935242)...)
	refvec = append(refvec, rutils.BytesFromFloat64LE(10.0)...)
	err = v.writeRegisterSPI(ctx, referenceVectorConfiguration, refvec)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't set reference vector")
	}
	// enforce acceleration tuinning and reduce "trust" in acceleration data
	accVpeTunning := []byte{}
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(3)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(3)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(3)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, rutils.BytesFromFloat32LE(10)...)

	err = v.writeRegisterSPI(ctx, vpeAccTunning, accVpeTunning)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't set vpe adaptive tunning")
	}
	err = v.writeRegisterSPI(ctx, deltaVDeltaThetaConfig, []byte{0, 0, 0, 0, 0, 0})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't configure deltaV register")
	}
	// tare the heading
	err = v.vectornavTareSPI(ctx)
	if err != nil {
		return nil, err
	}
	smp := uint(100)
	if v.polling > 0 {
		smp = uint(v.polling)
	}

	// compensate for acceleration bias due to misalignement
	err = v.compensateAccelBias(ctx, smp)
	if err != nil {
		return nil, err
	}
	// compensate for constant DV bias in mesurament
	err = v.compensateDVBias(ctx, smp)
	if err != nil {
		return nil, err
	}
	var cancelCtx context.Context
	cancelCtx, v.cancelFunc = context.WithCancel(ctx)
	// optionally start a polling goroutine
	if pfreq > 0 {
		logger.Debugf("vecnav: will pool at %d Hz", pfreq)
		waitCh := make(chan struct{})
		s := 1.0 / float64(pfreq)
		v.activeBackgroundWorkers.Add(1)
		rdkutils.PanicCapturingGo(func() {
			defer v.activeBackgroundWorkers.Done()
			timer := time.NewTicker(time.Duration(s * float64(time.Second)))
			defer timer.Stop()
			close(waitCh)
			for {
				select {
				case <-cancelCtx.Done():
					return
				default:
				}
				select {
				case <-cancelCtx.Done():
					return
				case <-timer.C:
					v.mu.Lock()
					err := v.getReadings(ctx)
					if err != nil {
						return
					}
					v.mu.Unlock()
				}
			}
		})
		<-waitCh
	}
	return v, nil
}

func (v *vectornav) ReadAngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.angularVelocity, nil
}

func (v *vectornav) ReadAcceleration(ctx context.Context) (r3.Vector, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.acceleration, nil
}

func (v *vectornav) ReadOrientation(ctx context.Context) (spatialmath.Orientation, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	return &v.orientation, nil
}

func (v *vectornav) getReadings(ctx context.Context) error {
	out, err := v.readRegisterSPI(ctx, yawPitchRollMagAccGyro, 48)
	if err != nil {
		return err
	}
	v.orientation.Yaw = rutils.DegToRad(float64(rutils.Float32FromBytesLE(out[0:4])))
	v.orientation.Pitch = rutils.DegToRad(float64(rutils.Float32FromBytesLE(out[4:8])))
	v.orientation.Roll = rutils.DegToRad(float64(rutils.Float32FromBytesLE(out[8:12])))
	// unit gauss
	v.magnetometer.X = float64(rutils.Float32FromBytesLE(out[12:16]))
	v.magnetometer.Y = float64(rutils.Float32FromBytesLE(out[16:20]))
	v.magnetometer.Z = float64(rutils.Float32FromBytesLE(out[20:24]))
	// unit mm/s^2
	v.acceleration.X = float64(rutils.Float32FromBytesLE(out[24:28])) * 1000
	v.acceleration.Y = float64(rutils.Float32FromBytesLE(out[28:32])) * 1000
	v.acceleration.Z = float64(rutils.Float32FromBytesLE(out[32:36])) * 1000
	// unit rad/s
	v.angularVelocity.X = rutils.RadToDeg(float64(rutils.Float32FromBytesLE(out[36:40])))
	v.angularVelocity.Y = rutils.RadToDeg(float64(rutils.Float32FromBytesLE(out[40:44])))
	v.angularVelocity.Z = rutils.RadToDeg(float64(rutils.Float32FromBytesLE(out[44:48])))
	dv, err := v.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
	if err != nil {
		return err
	}
	// unit deg/s
	v.dTheta.X = float64(rutils.Float32FromBytesLE(dv[4:8]))
	v.dTheta.Y = float64(rutils.Float32FromBytesLE(dv[8:12]))
	v.dTheta.Z = float64(rutils.Float32FromBytesLE(dv[12:16]))
	// unit m/s
	v.dV.X = float64(rutils.Float32FromBytesLE(dv[16:20])) - v.bdVX
	v.dV.Y = float64(rutils.Float32FromBytesLE(dv[20:24])) - v.bdVY
	v.dV.Z = float64(rutils.Float32FromBytesLE(dv[24:28])) - v.bdVZ
	// unit s
	v.dt = rutils.Float32FromBytesLE(dv[0:4])
	return nil
}

func (v *vectornav) GetReadings(ctx context.Context) ([]interface{}, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.polling == 0 {
		err := v.getReadings(ctx)
		if err != nil {
			return nil, err
		}
	}
	return []interface{}{
		v.angularVelocity,
		v.orientation,
		v.acceleration,
		v.dV,
		v.dt,
		v.dTheta,
	}, nil
}

func (v *vectornav) readRegisterSPI(ctx context.Context, reg vectornavRegister, readLen uint) ([]byte, error) {
	v.spiMu.Lock()
	defer v.spiMu.Unlock()
	if v.busClosed {
		return nil, errors.New("C=cannot read spi register the bus is closed")
	}
	hnd, err := v.bus.OpenHandle()
	if err != nil {
		return nil, err
	}
	cmd := []byte{byte(vectorNavSPIRead), byte(reg), 0, 0}
	_, err = hnd.Xfer(ctx, uint(v.speed), v.cs, 3, cmd)
	if err != nil {
		return nil, err
	}
	rdkutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	cmd = make([]byte, readLen+4)
	out, err := hnd.Xfer(ctx, uint(v.speed), v.cs, 3, cmd)
	if err != nil {
		return nil, err
	}
	if out[3] != 0 {
		return nil, errors.Errorf("vectornav read error returned %d speed was %d", out[3], v.speed)
	}
	err = hnd.Close()
	if err != nil {
		return nil, err
	}
	rdkutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	return out[4:], nil
}

func (v *vectornav) writeRegisterSPI(ctx context.Context, reg vectornavRegister, data []byte) error {
	v.spiMu.Lock()
	defer v.spiMu.Unlock()
	if v.busClosed {
		return errors.New("Cannot write spi register the bus is closed")
	}
	hnd, err := v.bus.OpenHandle()
	if err != nil {
		return err
	}
	cmd := []byte{byte(vectorNavSPIWrite), byte(reg), 0, 0}
	cmd = append(cmd, data...)
	_, err = hnd.Xfer(ctx, uint(v.speed), v.cs, 3, cmd)
	if err != nil {
		return err
	}
	rdkutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	cmd = make([]byte, len(data)+4)
	out, err := hnd.Xfer(ctx, uint(v.speed), v.cs, 3, cmd)
	if err != nil {
		return err
	}
	if out[3] != 0 {
		return errors.Errorf("vectornav write error returned %d", out[3])
	}
	err = hnd.Close()
	if err != nil {
		return err
	}
	rdkutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	return nil
}

func (v *vectornav) vectornavTareSPI(ctx context.Context) error {
	v.spiMu.Lock()
	defer v.spiMu.Unlock()
	if v.busClosed {
		return errors.New("Cannot write spi register the bus is closed")
	}
	hnd, err := v.bus.OpenHandle()
	if err != nil {
		return err
	}
	cmd := []byte{byte(vectorNavSPITare), 0, 0, 0}
	_, err = hnd.Xfer(ctx, uint(v.speed), v.cs, 3, cmd)
	if err != nil {
		return err
	}
	rdkutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	cmd = []byte{0, 0, 0, 0}
	out, err := hnd.Xfer(ctx, uint(v.speed), v.cs, 3, cmd)
	if err != nil {
		return err
	}
	if out[3] != 0 {
		return errors.Errorf("vectornav write error returned %d", out[3])
	}
	err = hnd.Close()
	if err != nil {
		return err
	}
	rdkutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	return nil
}

func (v *vectornav) compensateAccelBias(ctx context.Context, smpSize uint) error {
	var msg []byte
	msg = append(msg, rutils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(1.0)...)

	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	err := v.writeRegisterSPI(ctx, accCompensationConfiguration, msg)
	if err != nil {
		return errors.Wrap(err, "couldn't write the acceleration compensation register")
	}
	mdlG, err := v.readRegisterSPI(ctx, magAccRefVectors, 24)
	if err != nil {
		return errors.Wrap(err, "couldn't calculate acceleration bias")
	}
	accZ := rutils.Float32FromBytesLE(mdlG[20:24])
	var accMX, accMY, accMZ float32
	for i := uint(0); i < smpSize; i++ {
		acc, err := v.readRegisterSPI(ctx, acceleration, 12)
		if err != nil {
			return errors.Wrap(err, "error reading acceleration register during bias compensation")
		}
		accMX += rutils.Float32FromBytesLE(acc[0:4])
		accMY += rutils.Float32FromBytesLE(acc[4:8])
		accMZ += rutils.Float32FromBytesLE(acc[8:12])
		if !rdkutils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("error in context during acceleration compensation")
		}
	}
	accMX /= float32(smpSize)
	accMY /= float32(smpSize)
	accMZ /= float32(smpSize)
	msg = []byte{}
	msg = append(msg, rutils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, rutils.BytesFromFloat32LE(1.0)...)

	msg = append(msg, rutils.BytesFromFloat32LE(accMX)...)
	msg = append(msg, rutils.BytesFromFloat32LE(accMY)...)
	msg = append(msg, rutils.BytesFromFloat32LE(accZ+accMZ)...)

	err = v.writeRegisterSPI(ctx, accCompensationConfiguration, msg)
	if err != nil {
		return errors.Wrap(err, "could not write the acceleration register")
	}
	v.logger.Infof("Acceleration compensated with %1.6f %1.6f %1.6f ref accZ %1.6f", accMX, accMY, accMZ, accZ)
	return nil
}

func (v *vectornav) compensateDVBias(ctx context.Context, smpSize uint) error {
	var bX, bY, bZ float32
	_, err := v.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
	if err != nil {
		return errors.Wrap(err, "error reading dV register during bias compensation")
	}
	dt := 10 * time.Millisecond
	if v.polling > 0 {
		s := 1.0 / float64(v.polling)
		dt = time.Duration(s * float64(time.Second))
	}
	for i := uint(0); i < smpSize; i++ {
		if !rdkutils.SelectContextOrWait(ctx, dt) {
			return errors.New("error in context during Dv compensation")
		}
		dv, err := v.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
		if err != nil {
			return errors.Wrap(err, "error reading dV register during bias compensation")
		}
		bX += rutils.Float32FromBytesLE(dv[16:20])
		bY += rutils.Float32FromBytesLE(dv[20:24])
		bZ += rutils.Float32FromBytesLE(dv[24:28])
	}
	v.bdVX = float64(bX) / float64(smpSize)
	v.bdVY = float64(bY) / float64(smpSize)
	v.bdVZ = float64(bZ) / float64(smpSize)
	v.logger.Infof("velocity bias compensated with %1.6f %1.6f %1.6f",
		v.bdVX, v.bdVY, v.bdVZ)
	return nil
}

func (v *vectornav) Close() {
	v.logger.Debug("closing vecnav")
	v.cancelFunc()
	v.busClosed = true
	v.activeBackgroundWorkers.Wait()
}
