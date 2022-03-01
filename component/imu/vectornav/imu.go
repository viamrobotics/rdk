package vectornav

import (
	"context"
	"encoding/binary"
	"math"
	"sync"
	"time"

	"github.com/golang/geo/r3"
	"github.com/pkg/errors"

	"github.com/edaniels/golog"

	"go.viam.com/rdk/component/board"
	"go.viam.com/rdk/component/imu"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/robot"
	"go.viam.com/rdk/spatialmath"
	rdkutils "go.viam.com/utils"
)

const model = "vectornav"

func init() {
	registry.RegisterComponent(imu.Subtype, model, registry.Component{
		Constructor: func(ctx context.Context, r robot.Robot, config config.Component, logger golog.Logger) (interface{}, error) {
			return NewVectornav(r, config, logger)
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
	temp            float32
	pressure        float32
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
	vectorNavSPIRead          uint = 1
	vectorNavSPIWrite         uint = 2
	vectorNavSPIWriteSettings uint = 3
)

type vectornavRegister uint

const (
	modelNumber                    vectornavRegister = 1
	hardwareRevision               vectornavRegister = 2
	serialNumber                   vectornavRegister = 3
	firmwareVersion                vectornavRegister = 4
	imuMeasurements                vectornavRegister = 54
	deltaVDeltaTheta               vectornavRegister = 80
	deltaVDeltaThetaConfig         vectornavRegister = 82
	yawPitchRoll                   vectornavRegister = 8
	attitudeQuaternions            vectornavRegister = 9
	yawPitchRollMagAccGyro         vectornavRegister = 27
	quaternionMagAccGyro           vectornavRegister = 15
	magnetic                       vectornavRegister = 17
	acceleration                   vectornavRegister = 18
	gyros                          vectornavRegister = 19
	magAccGyro                     vectornavRegister = 20
	yawPitchRollTrueBodyAccGyro    vectornavRegister = 239
	velocityCompensationMeasurment vectornavRegister = 50
	referenceVectorConfiguration   vectornavRegister = 83
	magAccRefVectors               vectornavRegister = 21
	accCompensationConfiguration   vectornavRegister = 25
)

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
	v.orientation.Yaw = float64(Float32frombytes(out[0:4]))
	v.orientation.Pitch = float64(Float32frombytes(out[4:8]))
	v.orientation.Roll = float64(Float32frombytes(out[8:12]))
	v.magnetometer.X = float64(Float32frombytes(out[12:16]))
	v.magnetometer.Y = float64(Float32frombytes(out[16:20]))
	v.magnetometer.Z = float64(Float32frombytes(out[20:24]))
	v.acceleration.X = float64(Float32frombytes(out[24:28]))
	v.acceleration.Y = float64(Float32frombytes(out[28:32]))
	v.acceleration.Z = float64(Float32frombytes(out[32:36]))
	v.angularVelocity.X = float64(Float32frombytes(out[36:40]))
	v.angularVelocity.Y = float64(Float32frombytes(out[40:44]))
	v.angularVelocity.Z = float64(Float32frombytes(out[44:48]))
	dv, err := v.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
	if err != nil {
		return err
	}
	v.dTheta.X = float64(Float32frombytes(dv[4:8]))
	v.dTheta.Y = float64(Float32frombytes(dv[8:12]))
	v.dTheta.Z = float64(Float32frombytes(dv[12:16]))
	v.dV.X = float64(Float32frombytes(dv[16:20])) - v.bdVX
	v.dV.Y = float64(Float32frombytes(dv[20:24])) - v.bdVY
	v.dV.Z = float64(Float32frombytes(dv[24:28])) - v.bdVZ
	v.dt = Float32frombytes(dv[0:4])
	if err != nil {
		return err
	}
	return nil
}

func (v *vectornav) GetReadings(ctx context.Context) ([]interface{}, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.polling == 0 {
		v.getReadings(ctx)
	}
	return []interface{}{v.angularVelocity,
		v.orientation,
		v.acceleration,
		v.dV,
		v.dt,
		v.dTheta,
		v.pressure,
		v.temp}, nil
}

func (v *vectornav) readRegisterSPI(ctx context.Context, reg vectornavRegister, len uint) ([]byte, error) {
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
	rdkutils.SelectContextOrWait(ctx, 200*time.Microsecond)
	cmd = make([]byte, len+4)
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
	rdkutils.SelectContextOrWait(ctx, 200*time.Microsecond)
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
	rdkutils.SelectContextOrWait(ctx, 200*time.Microsecond)
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
	rdkutils.SelectContextOrWait(ctx, 200*time.Microsecond)
	return nil
}
func bytesFromFloat64(v float64) []byte {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], math.Float64bits(v))
	return b[:]
}
func bytesFromFloat32(v float32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], math.Float32bits(v))
	return b[:]
}
func bytesFromUint32(v uint32) []byte {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	return b[:]
}
func Float32frombytes(bytes []byte) float32 {
	bits := binary.LittleEndian.Uint32(bytes)
	float := math.Float32frombits(bits)
	return float
}
func (v *vectornav) compensateAccelBias(ctx context.Context) error {
	var msg []byte
	msg = append(msg, bytesFromFloat32(1.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)

	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(1.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)

	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(1.0)...)

	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)
	err := v.writeRegisterSPI(ctx, accCompensationConfiguration, msg)
	if err != nil {
		return errors.Wrap(err, "couldn't write the acceleration compensation register")
	}
	mdlG, err := v.readRegisterSPI(ctx, magAccRefVectors, 24)
	if err != nil {
		return errors.Wrap(err, "couldn't calculate accel bias")
	}
	accZ := Float32frombytes(mdlG[20:24])
	var accMX float32
	var accMY float32
	var accMZ float32
	accMX = 0.0
	accMY = 0.0
	accMZ = 0.0
	for i := 0; i < 100; i++ {
		acc, err := v.readRegisterSPI(ctx, acceleration, 12)
		if err != nil {
			return errors.Wrap(err, "error reading acceleration register during bias compensation")
		}
		accMX += Float32frombytes(acc[0:4])
		accMY += Float32frombytes(acc[4:8])
		accMZ += Float32frombytes(acc[8:12])
		if !rdkutils.SelectContextOrWait(ctx, 10*time.Millisecond) {
			return errors.New("error in context during acceleration compensation")
		}
	}
	accMX = accMX / 100.0
	accMY = accMY / 100.0
	accMZ = accMZ / 100.0
	msg = []byte{}
	msg = append(msg, bytesFromFloat32(1.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)

	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(1.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)

	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(0.0)...)
	msg = append(msg, bytesFromFloat32(1.0)...)

	msg = append(msg, bytesFromFloat32(accMX)...)
	msg = append(msg, bytesFromFloat32(accMY)...)
	msg = append(msg, bytesFromFloat32(accZ+accMZ)...)

	err = v.writeRegisterSPI(ctx, accCompensationConfiguration, msg)
	if err != nil {
		return errors.Wrap(err, "couldn't write the acceleration register")
	}
	v.logger.Infof("Acc compensated with %1.6f %1.6f %1.6f ref accZ %1.6f", accMX, accMY, accMZ, accZ)
	return nil
}
func (v *vectornav) compensateDVBias(ctx context.Context) error {
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
	for i := 0; i < 100; i++ {
		if !rdkutils.SelectContextOrWait(ctx, dt) {
			return errors.New("error in context during Dv compensation")
		}
		dv, err := v.readRegisterSPI(ctx, deltaVDeltaTheta, 28)
		if err != nil {
			return errors.Wrap(err, "error reading dV register during bias compensation")
		}
		bX += Float32frombytes(dv[16:20])
		bY += Float32frombytes(dv[20:24])
		bZ += Float32frombytes(dv[24:28])
	}
	v.bdVX = float64(bX) / 100.0
	v.bdVY = float64(bY) / 100.0
	v.bdVZ = float64(bZ) / 100.0
	v.logger.Infof("velocity bias compensated with %1.6f %1.6f %1.6f",
		v.bdVX, v.bdVY, v.bdVZ)
	return nil
}
func NewVectornav(r robot.Robot, config config.Component, logger golog.Logger) (imu.IMU, error) {
	logger.Debug("building a vectornav IMU")
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
	mdl, err := v.readRegisterSPI(context.Background(), modelNumber, 24)
	if err != nil {
		return nil, err
	}
	sn, err := v.readRegisterSPI(context.Background(), serialNumber, 4)
	if err != nil {
		return nil, err
	}
	fwver, err := v.readRegisterSPI(context.Background(), firmwareVersion, 4)
	if err != nil {
		return nil, err
	}
	logger.Debugf("model detected %s sn %d %d.%d.%d.%d", string(mdl), binary.LittleEndian.Uint32(sn), fwver[0], fwver[1], fwver[2], fwver[3])

	refvec := []byte{1, 1, 0, 0}
	refvec = append(refvec, bytesFromUint32(1000)...)
	refvec = append(refvec, bytesFromFloat32(2010.0)...)
	refvec = append(refvec, []byte{0, 0, 0, 0}...)
	refvec = append(refvec, bytesFromFloat64(40.730610)...)
	refvec = append(refvec, bytesFromFloat64(-73.935242)...)
	refvec = append(refvec, bytesFromFloat64(10.0)...)
	err = v.writeRegisterSPI(context.Background(), referenceVectorConfiguration, refvec)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't set reference vector")
	}
	err = v.writeRegisterSPI(context.Background(), deltaVDeltaThetaConfig, []byte{0, 1, 1, 0, 0, 0})
	if err != nil {
		return nil, errors.Wrap(err, "couldn't configure deltaV register")
	}
	err = v.compensateAccelBias(context.Background())
	if err != nil {
		return nil, err
	}
	err = v.compensateDVBias(context.Background())
	if err != nil {
		return nil, err
	}
	var ctx context.Context
	ctx, v.cancelFunc = context.WithCancel(context.Background())
	if pfreq > 0 {
		logger.Debugf("vecnav: will pool at %d Hz", pfreq)
		waitCh := make(chan struct{})
		s := 1.0 / float64(pfreq)
		v.activeBackgroundWorkers.Add(1)
		rdkutils.PanicCapturingGo(func() {
			defer v.activeBackgroundWorkers.Done()
			timer := time.NewTicker(time.Duration(s * float64(time.Second)))
			close(waitCh)
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					v.mu.Lock()
					v.getReadings(ctx)
					v.mu.Unlock()
				}
			}
		})
		<-waitCh
	}
	return v, nil
}

func (v *vectornav) Close() {
	v.logger.Debug("closing vecnav")
	v.cancelFunc()
	v.busClosed = true
	v.activeBackgroundWorkers.Wait()
}
