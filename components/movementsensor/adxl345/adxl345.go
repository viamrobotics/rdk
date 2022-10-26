package adxl345

// TODO: remove unused imports
import (
	"context"
	"encoding/binary"
	"sync"

	"github.com/edaniels/golog"
	"github.com/golang/geo/r3"
	geo "github.com/kellydunn/golang-geo"
	"go.viam.com/utils"
	"github.com/pkg/errors"

	"go.viam.com/rdk/components/board"
	//"go.viam.com/rdk/components/generic"
	"go.viam.com/rdk/components/movementsensor"
	"go.viam.com/rdk/config"
	"go.viam.com/rdk/registry"
	"go.viam.com/rdk/spatialmath"
	//rutils "go.viam.com/rdk/utils"
)

const modelName = "adxl345"

type AttrConfig struct {
	BoardName              string `json:"board"`
	BusId                  string `json: "bus_id"`
	UseAlternateI2CAddress bool   `json: "use_alternate_i2c_address"`
}

// Validate ensures all parts of the config are valid.
func (cfg *AttrConfig) Validate(path string) error {
	if cfg.BoardName == "" {
		return utils.NewConfigValidationFieldRequiredError(path, "board")
	}
	return nil
}

func init() {
	registry.RegisterComponent(movementsensor.Subtype, modelName, registry.Component{
		Constructor: func(
			ctx context.Context,
			deps registry.Dependencies,
			cfg config.Component,
			logger golog.Logger,
		) (interface{}, error) {
			return NewAdxl345(ctx, deps, cfg, logger)
		},
	})

	config.RegisterComponentAttributeMapConverter(movementsensor.SubtypeName, modelName,
		func(attributes config.AttributeMap) (interface{}, error) {
			var attr AttrConfig
			return config.TransformAttributeMapToStruct(&attr, attributes)
		},
		&AttrConfig{})
}

type adxl345 struct {
	b          board.Board
	bus        board.I2C
	i2cAddress byte
	mu         sync.Mutex

	logger     golog.Logger
}

func NewAdxl345(
	ctx context.Context,
	deps registry.Dependencies,
	rawConfig config.Component,
	logger golog.Logger,
) (movementsensor.MovementSensor, error) {
	cfg := rawConfig.ConvertedAttributes.(*AttrConfig)
	b, err := board.FromDependencies(deps, cfg.BoardName)
	if err != nil {
		return nil, err
	}
	localB, ok := b.(board.LocalBoard)
	if !ok {
		return nil, errors.Errorf("board %s is not local", cfg.BoardName)
	}
	bus, ok := localB.I2CByName(cfg.BusId)
	if !ok {
		return nil, errors.Errorf("can't find I2C bus '%s' for ADXL345 sensor", cfg.BusId)
	}

	var address byte
	if cfg.UseAlternateI2CAddress {
		address = 0x1D
	} else {
		address = 0x53
	}

	sensor := &adxl345{
		b: b,
		bus: bus,
		i2cAddress: address,
		logger: logger,
	}

	// To check that we're able to talk to the chip, we should be able to read register 0 and get
	// back the device ID (0xE5).
	deviceId, err := sensor.readByte(0, ctx)
	if err != nil {
		return nil, errors.Errorf("can't read from I2C address {} on bus {} of board {}: '{}'",
		                          address, cfg.BusId, cfg.BoardName, err.Error())
	}
	if deviceId != 0xE5 {
		return nil, errors.Errorf("unexpected I2C device instead of ADXL345: deviceID '{}'",
		                          deviceId)
	}

	// The chip starts out in standby mode. Set it to measurement mode so we can get data from it.
	// To do this, we set the Power Control register (0x2D) to turn on the 8's bit.
	err = sensor.writeByte(0x2D, 0x08, ctx)
	if (err != nil) {
		return nil, errors.Errorf("unable to put ADXL345 into measurement mode: '{}'", err.Error())
	}

	return sensor, nil
}

func toSignedValue(data []byte) int16 {
	// The registers on the chip are laid out as little-endian: the first register is the less
	// significant byte.
	return int16(binary.LittleEndian.Uint16(data))
}

func (adxl *adxl345) readByte(register byte, ctx context.Context) (byte, error) {
	result, err := adxl.readBlock(register, 1, ctx)
	if err != nil {
		return 0, err
	}
	return result[0], err
}

func (adxl *adxl345) readBlock(register byte, length int, ctx context.Context) ([]byte, error) {
	handle, err := adxl.bus.OpenHandle(adxl.i2cAddress)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := handle.Close()
		if err != nil {
			adxl.logger.Error(err)
		}
	}()

	results := make([]byte, length)
	results, err = handle.Read(ctx, length)
	return results, err
}

func (adxl *adxl345) writeByte(register byte, value byte, ctx context.Context) error {
	handle, err := adxl.bus.OpenHandle(adxl.i2cAddress)
	if err != nil {
		return err
	}
	defer func() {
		err := handle.Close()
		if err != nil {
			adxl.logger.Error(err)
		}
	}()

	return handle.WriteByteData(ctx, register, value)
}

func (adxl *adxl345) Position(ctx context.Context) (*geo.Point, float64, error) {
	return geo.NewPoint(0, 0), 0, movementsensor.ErrMethodUnimplementedPosition
}

func (adxl *adxl345) LinearVelocity(ctx context.Context) (r3.Vector, error) {
	return r3.Vector{}, movementsensor.ErrMethodUnimplementedLinearVelocity
}

func (adxl *adxl345) AngularVelocity(ctx context.Context) (spatialmath.AngularVelocity, error) {
	return spatialmath.AngularVelocity{}, movementsensor.ErrMethodUnimplementedAngularVelocity
}

func (adxl *adxl345) CompassHeading(ctx context.Context) (float64, error) {
	return 0, movementsensor.ErrMethodUnimplementedCompassHeading
}

func (adxl *adxl345) Orientation(ctx context.Context) (spatialmath.Orientation, error) {
	// TODO: implement this one?
	return nil, movementsensor.ErrMethodUnimplementedOrientation
}

func (adxl *adxl345) Properties(ctx context.Context) (*movementsensor.Properties, error) {
	return &movementsensor.Properties{
		LinearVelocitySupported:  false,
		AngularVelocitySupported: false,
		OrientationSupported:     false,
		PositionSupported:        false,
		CompassHeadingSupported:  false,
	}, nil
}

func (adxl *adxl345) Accuracy(ctx context.Context) (map[string]float32, error) {
	return map[string]float32{}, movementsensor.ErrMethodUnimplementedAccuracy
}

func (adxl *adxl345) Readings(ctx context.Context) (map[string]interface{}, error) {
	// TODO: implement this one, too.
	return nil, nil
}

func (u *adxl345) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	// TODO: implement this.
	return nil, nil
}

func (adxl *adxl345) Close() {
	// Put the chip into standby mode by setting the Power Control register (0x2D) to 0.
	err := adxl.writeByte(0x2D, 0x00, nil)
	if err != nil {
		adxl.logger.Error(err)
	}
}
