//go:build linux

// Package imuvectornav implements a component for a vectornav IMU.
package imuvectornav

import (
	"context"
	"encoding/binary"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"github.com/pkg/errors"
	goutils "go.viam.com/utils"

	"go.viam.com/rdk/components/board"
	"go.viam.com/rdk/components/board/genericlinux"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/spatialmath"
	"go.viam.com/rdk/utils"
)

var model = resource.DefaultModelFamily.WithModel("imu-vectornav")

// Config is used for converting a vectornav IMU MovementSensor config attributes.
type Config struct {
	SPI   string `json:"spi_bus"`
	Speed *int   `json:"spi_baud_rate"`
	Pfreq *int   `json:"polling_freq_hz"`
	CSPin string `json:"chip_select_pin"`
}

// Validate ensures all parts of the config are valid.
func (cfg *Config) Validate(path string) ([]string, error) {
	var deps []string
	if cfg.SPI == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "spi")
	}

	if cfg.Speed == nil {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "spi_baud_rate")
	}

	if cfg.Pfreq == nil {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "polling_freq_hz")
	}

	if cfg.CSPin == "" {
		return nil, goutils.NewConfigValidationFieldRequiredError(path, "cs_pin (chip select pin)")
	}
	return deps, nil
}

func init() {
	resource.RegisterComponent(movementsensor.API, model, resource.Registration[movementsensor.MovementSensor, *Config]{
		Constructor: newVectorNav,
	})
}

type vectornav struct {
	resource.Named
	resource.AlwaysRebuild
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
	logger                  logging.Logger
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

// newVectorNav connect and set up a vectornav IMU over SPI.
// Will also compensate for acceleration and delta velocity bias over one second so be
// sure the IMU is still when calling this function.
func newVectorNav(
	ctx context.Context,
	deps resource.Dependencies,
	conf resource.Config,
	logger logging.Logger,
) (movementsensor.MovementSensor, error) {
	newConf, err := resource.NativeConfig[*Config](conf)
	if err != nil {
		return nil, err
	}

	speed := *newConf.Speed
	if speed == 0 {
		speed = 8000000
	}

	pfreq := *newConf.Pfreq
	v := &vectornav{
		Named:     conf.ResourceName().AsNamed(),
		bus:       genericlinux.NewSpiBus(newConf.SPI),
		logger:    logger,
		cs:        newConf.CSPin,
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
	logger.Debugf(
		"model detected %s sn %d %d.%d.%d.%d",
		string(mdl),
		binary.LittleEndian.Uint32(sn),
		fwver[0],
		fwver[1],
		fwver[2],
		fwver[3],
	)

	// set imu location to New York for the WGM model
	refvec := []byte{1, 1, 0, 0}
	refvec = append(refvec, utils.BytesFromUint32LE(1000)...)
	refvec = append(refvec, utils.BytesFromFloat32LE(2010.0)...)
	refvec = append(refvec, []byte{0, 0, 0, 0}...)
	refvec = append(refvec, utils.BytesFromFloat64LE(40.730610)...)
	refvec = append(refvec, utils.BytesFromFloat64LE(-73.935242)...)
	refvec = append(refvec, utils.BytesFromFloat64LE(10.0)...)
	err = v.writeRegisterSPI(ctx, referenceVectorConfiguration, refvec)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't set reference vector")
	}
	// enforce acceleration tuinning and reduce "trust" in acceleration data
	accVpeTunning := []byte{}
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(3)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(3)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(3)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(10)...)
	accVpeTunning = append(accVpeTunning, utils.BytesFromFloat32LE(10)...)

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
	cancelCtx, v.cancelFunc = context.WithCancel(context.Background())
	// optionally start a polling goroutine
	if pfreq > 0 {
		logger.Debugf("vecnav: will pool at %d Hz", pfreq)
		waitCh := make(chan struct{})
		s := 1.0 / float64(pfreq)
		v.activeBackgroundWorkers.Add(1)
		goutils.PanicCapturingGo(func() {
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
					err := v.getReadings(ctx)
					if err != nil {
						return
					}
				}
			}
		})
		<-waitCh
	}
	return v, nil
}

func (vn *vectornav) AngularVelocity(ctx context.Context, extra map[string]interface{}) (spatialmath.AngularVelocity, error) {
	vn.mu.Lock()
	defer vn.mu.Unlock()
	return vn.angularVelocity, nil
}

func (vn *vectornav) LinearAcceleration(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	vn.mu.Lock()
	defer vn.mu.Unlock()
	return vn.acceleration, nil
}

func (vn *vectornav) Orientation(ctx context.Context, extra map[string]interface{}) (spatialmath.Orientation, error) {
	vn.mu.Lock()
	defer vn.mu.Unlock()
	return &vn.orientation, nil
}

func (vn *vectornav) CompassHeading(ctx context.Context, extra map[string]interface{}) (float64, error) {
	vn.mu.Lock()
	defer vn.mu.Unlock()
	return vn.orientation.Yaw, nil
}

func (vn *vectornav) LinearVelocity(ctx context.Context, extra map[string]interface{}) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
}

func (vn *vectornav) Position(ctx context.Context, extra map[string]interface{}) (*geo.Point, float64, error) {
	return nil, 0, movementsensor.ErrMethodUnimplementedPosition
}

func (vn *vectornav) Accuracy(ctx context.Context, extra map[string]interface{}) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (vn *vectornav) GetMagnetometer(ctx context.Context) (r3.Vector, error) {
	vn.mu.Lock()
	defer vn.mu.Unlock()
	return vn.magnetometer, nil
}

func (vn *vectornav) Properties(ctx context.Context, extra map[string]interface{}) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		AngularVelocitySupported:    true,
		OrientationSupported:        true,
		LinearAccelerationSupported: true,
	}, nil
}

func (vn *vectornav) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	return movementsensor.Readings(ctx, vn, extra)
}

func (vn *vectornav) getReadings(ctx context.Context) error {
	out, err := vn.readRegisterSPI(ctx, yawPitchRollMagAccGyro, 48)
	if err != nil {
		return err
	}
	vn.mu.Lock()
	defer vn.mu.Unlock()
	vn.orientation.Yaw = utils.DegToRad(float64(utils.Float32FromBytesLE(out[0:4])))
	vn.orientation.Pitch = utils.DegToRad(float64(utils.Float32FromBytesLE(out[4:8])))
	vn.orientation.Roll = utils.DegToRad(float64(utils.Float32FromBytesLE(out[8:12])))
	// unit gauss
	vn.magnetometer.X = float64(utils.Float32FromBytesLE(out[12:16]))
	vn.magnetometer.Y = float64(utils.Float32FromBytesLE(out[16:20]))
	vn.magnetometer.Z = float64(utils.Float32FromBytesLE(out[20:24]))
	// unit mm/s^2
	vn.acceleration.X = float64(utils.Float32FromBytesLE(out[24:28])) * 1000
	vn.acceleration.Y = float64(utils.Float32FromBytesLE(out[28:32])) * 1000
	vn.acceleration.Z = float64(utils.Float32FromBytesLE(out[32:36])) * 1000
	// unit rad/s
	vn.angularVelocity.X = utils.RadToDeg(float64(utils.Float32FromBytesLE(out[36:40])))
	vn.angularVelocity.Y = utils.RadToDeg(float64(utils.Float32FromBytesLE(out[40:44])))
	vn.angularVelocity.Z = utils.RadToDeg(float64(utils.Float32FromBytesLE(out[44:48])))
	dv, err := vn.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
	if err != nil {
		return err
	}
	// unit deg/s
	vn.dTheta.X = float64(utils.Float32FromBytesLE(dv[4:8]))
	vn.dTheta.Y = float64(utils.Float32FromBytesLE(dv[8:12]))
	vn.dTheta.Z = float64(utils.Float32FromBytesLE(dv[12:16]))
	// unit m/s
	vn.dV.X = float64(utils.Float32FromBytesLE(dv[16:20])) - vn.bdVX
	vn.dV.Y = float64(utils.Float32FromBytesLE(dv[20:24])) - vn.bdVY
	vn.dV.Z = float64(utils.Float32FromBytesLE(dv[24:28])) - vn.bdVZ
	// unit s
	vn.dt = utils.Float32FromBytesLE(dv[0:4])
	return nil
}

func (vn *vectornav) readRegisterSPI(ctx context.Context, reg vectornavRegister, readLen uint) ([]byte, error) {
	vn.spiMu.Lock()
	defer vn.spiMu.Unlock()
	if vn.busClosed {
		return nil, errors.New("C=cannot read spi register the bus is closed")
	}
	hnd, err := vn.bus.OpenHandle()
	if err != nil {
		return nil, err
	}
	cmd := []byte{byte(vectorNavSPIRead), byte(reg), 0, 0}
	_, err = hnd.Xfer(ctx, uint(vn.speed), vn.cs, 3, cmd)
	if err != nil {
		return nil, err
	}
	goutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	cmd = make([]byte, readLen+4)
	out, err := hnd.Xfer(ctx, uint(vn.speed), vn.cs, 3, cmd)
	if err != nil {
		return nil, err
	}
	if out[3] != 0 {
		return nil, errors.Errorf("vectornav read error returned %d speed was %d", out[3], vn.speed)
	}
	err = hnd.Close()
	if err != nil {
		return nil, err
	}
	goutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	return out[4:], nil
}

func (vn *vectornav) writeRegisterSPI(ctx context.Context, reg vectornavRegister, data []byte) error {
	vn.spiMu.Lock()
	defer vn.spiMu.Unlock()
	if vn.busClosed {
		return errors.New("Cannot write spi register the bus is closed")
	}
	hnd, err := vn.bus.OpenHandle()
	if err != nil {
		return err
	}
	cmd := []byte{byte(vectorNavSPIWrite), byte(reg), 0, 0}
	cmd = append(cmd, data...)
	_, err = hnd.Xfer(ctx, uint(vn.speed), vn.cs, 3, cmd)
	if err != nil {
		return err
	}
	goutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	cmd = make([]byte, len(data)+4)
	out, err := hnd.Xfer(ctx, uint(vn.speed), vn.cs, 3, cmd)
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
	goutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	return nil
}

func (vn *vectornav) vectornavTareSPI(ctx context.Context) error {
	vn.spiMu.Lock()
	defer vn.spiMu.Unlock()
	if vn.busClosed {
		return errors.New("Cannot write spi register the bus is closed")
	}
	hnd, err := vn.bus.OpenHandle()
	if err != nil {
		return err
	}
	cmd := []byte{byte(vectorNavSPITare), 0, 0, 0}
	_, err = hnd.Xfer(ctx, uint(vn.speed), vn.cs, 3, cmd)
	if err != nil {
		return err
	}
	goutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	cmd = []byte{0, 0, 0, 0}
	out, err := hnd.Xfer(ctx, uint(vn.speed), vn.cs, 3, cmd)
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
	goutils.SelectContextOrWait(ctx, 110*time.Microsecond)
	return nil
}

func (vn *vectornav) compensateAccelBias(ctx context.Context, smpSize uint) error {
	var msg []byte
	msg = append(msg, utils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(1.0)...)

	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	err := vn.writeRegisterSPI(ctx, accCompensationConfiguration, msg)
	if err != nil {
		return errors.Wrap(err, "couldn't write the acceleration compensation register")
	}
	mdlG, err := vn.readRegisterSPI(ctx, magAccRefVectors, 24)
	if err != nil {
		return errors.Wrap(err, "couldn't calculate acceleration bias")
	}
	accZ := utils.Float32FromBytesLE(mdlG[20:24])
	var accMX, accMY, accMZ float32
	for i := uint(0); i < smpSize; i++ {
		acc, err := vn.readRegisterSPI(ctx, acceleration, 12)
		if err != nil {
			return errors.Wrap(err, "error reading acceleration register during bias compensation")
		}
		accMX += utils.Float32FromBytesLE(acc[0:4])
		accMY += utils.Float32FromBytesLE(acc[4:8])
		accMZ += utils.Float32FromBytesLE(acc[8:12])
		if !goutils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("error in context during acceleration compensation")
		}
	}
	accMX /= float32(smpSize)
	accMY /= float32(smpSize)
	accMZ /= float32(smpSize)
	msg = []byte{}
	msg = append(msg, utils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(1.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)

	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(0.0)...)
	msg = append(msg, utils.BytesFromFloat32LE(1.0)...)

	msg = append(msg, utils.BytesFromFloat32LE(accMX)...)
	msg = append(msg, utils.BytesFromFloat32LE(accMY)...)
	msg = append(msg, utils.BytesFromFloat32LE(accZ+accMZ)...)

	err = vn.writeRegisterSPI(ctx, accCompensationConfiguration, msg)
	if err != nil {
		return errors.Wrap(err, "could not write the acceleration register")
	}
	vn.logger.Infof("Acceleration compensated with %1.6f %1.6f %1.6f ref accZ %1.6f", accMX, accMY, accMZ, accZ)
	return nil
}

func (vn *vectornav) compensateDVBias(ctx context.Context, smpSize uint) error {
	var bX, bY, bZ float32
	_, err := vn.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
	if err != nil {
		return errors.Wrap(err, "error reading dV register during bias compensation")
	}
	dt := 10 * time.Millisecond
	if vn.polling > 0 {
		s := 1.0 / float64(vn.polling)
		dt = time.Duration(s * float64(time.Second))
	}
	for j := uint(0); j < smpSize; j++ {
		if !goutils.SelectContextOrWait(ctx, dt) {
			return errors.New("error in context during Dv compensation")
		}
		dv, err := vn.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
		if err != nil {
			return errors.Wrap(err, "error reading dV register during bias compensation")
		}
		bX += utils.Float32FromBytesLE(dv[16:20])
		bY += utils.Float32FromBytesLE(dv[20:24])
		bZ += utils.Float32FromBytesLE(dv[24:28])
	}
	vn.bdVX = float64(bX) / float64(smpSize)
	vn.bdVY = float64(bY) / float64(smpSize)
	vn.bdVZ = float64(bZ) / float64(smpSize)
	vn.logger.Infof("velocity bias compensated with %1.6f %1.6f %1.6f",
		vn.bdVX, vn.bdVY, vn.bdVZ)
	return nil
}

func (vn *vectornav) Close(ctx context.Context) error {
	vn.logger.Debug("closing vecnav imu")
	vn.cancelFunc()
	vn.busClosed = true
	vn.activeBackgroundWorkers.Wait()
	vn.logger.Debug("closed vecnav imu")
	return nil
}
