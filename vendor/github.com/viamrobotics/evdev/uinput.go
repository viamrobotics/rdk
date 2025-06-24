package evdev

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// UserInput wraps sending and receiving events to a user input (uinput)
// device.
type UserInput struct {
	fd *os.File

	syspath string

	id         ID
	name       string
	path       string
	effectsMax uint32

	events map[EventType]bool
}

// NewUserInput creates a new user input (uinput) device with the specified
// permissions.
func NewUserInput(perm os.FileMode, opts ...UserInputOption) (*UserInput, error) {
	var err error

	// setup
	u := &UserInput{
		name: "<none>",
	}

	// apply opts
	for _, o := range opts {
		o(u)
	}

	// open
	fd, err := os.OpenFile("/dev/uinput", syscall.O_WRONLY|syscall.O_NONBLOCK, perm)
	if err != nil {
		return nil, err
	}

	// init
	err = u.init(fd)
	if err != nil {
		fd.Close()
		return nil, err
	}
	return u, nil
}

// init wraps initializing a user input (uinput) device.
func (u *UserInput) init(fd *os.File) error {
	var err error

	// config
	config := uisetup{
		id:         u.id,
		effectsMax: u.effectsMax,
	}
	copy(config.name[:], u.name)

	// setup
	err = ioctl(fd.Fd(), uiDevSetup, unsafe.Pointer(&config))
	if err != nil {
		fd.Close()
		return err
	}

	// set path
	buf := append([]byte(u.path), 0)
	err = ioctl(fd.Fd(), uiSetPhys, unsafe.Pointer(&buf[0]))
	if err != nil {
		fd.Close()
		return err
	}

	// create
	err = ioctl(fd.Fd(), uiDevCreate, 0)
	if err != nil {
		fd.Close()
		return err
	}

	// read sys path
	u.path, err = ioctlString(fd.Fd(), uiGetSysname, 256)
	if err != nil {
		fd.Close()
		return err
	}

	u.fd = fd
	return nil
}

// Close closes the user input (uinput) device.
func (u *UserInput) Close() error {
	if u.fd != nil {
		fd := u.fd
		u.fd = nil
		err := ioctl(fd.Fd(), uiDevDestroy, 0)
		fd.Close()
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

// Path returns the path of the device.
func (u *UserInput) Path() string {
	return u.path
}

// UserInputOption is a user input (uinput) device option.
type UserInputOption func(*UserInput)

// WithID is a user input (uinput) device option to set the device ID.
func WithID(id ID) UserInputOption {
	return func(u *UserInput) {
		u.id = id
	}
}

// WithName is a user input (uinput) device option to set the device name.
func WithName(name string) UserInputOption {
	return func(u *UserInput) {
		u.name = name
	}
}

// WithPath is a user input (uinput) device option to set the device path.
func WithPath(path string) UserInputOption {
	return func(u *UserInput) {
		u.path = path
	}
}

// WithEventTypes is a user input (uinput) device option to set the device
// event types.
func WithEventTypes(events map[EventType]bool) UserInputOption {
	return func(u *UserInput) {
		/*var err error
		for _, typ := range types {
			err = ioctl(u.fd.Fd(), uiSetEvBit, typ)
			if err != nil {
				return err
			}
		}*/
		u.events = events
	}
}

// WithSyncTypes is a user input (uinput) device option to set the device
// sync event types.
func WithSyncTypes(types map[SyncType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithKeyTypes is a user input (uinput) device option to set the device
// key event types.
func WithKeyTypes(types map[KeyType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithRelativeTypes is a user input (uinput) device option to set the device
// relative event types.
func WithRelativeTypes(types map[RelativeType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithAbsoluteTypes is a user input (uinput) device option to set the device
// absolute event types.
func WithAbsoluteTypes(types map[AbsoluteType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithMiscTypes is a user input (uinput) device option to set the device misc
// event types.
func WithMiscTypes(types map[MiscType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithSwitchTypes is a user input (uinput) device option to set the device
// switch event types.
func WithSwitchTypes(types map[SwitchType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithLEDTypes is a user input (uinput) device option to set the device led
// event types.
func WithLEDTypes(types map[LEDType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithSoundTypes is a user input (uinput) device option to set the device
// sound event types.
func WithSoundTypes(types map[SoundType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithRepeatTypes is a user input (uinput) device option to set the device
// repeat event types.
func WithRepeatTypes(types map[RepeatType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithEffectTypes is a user input (uinput) device option to set the device
// force feedback effect event types.
func WithEffectTypes(types map[EffectType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithPowerTypes is a user input (uinput) device option to set the device
// power event types.
func WithPowerTypes(types map[PowerType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithEffectStatusTypes is a user input (uinput) device option to set the
// device force feedback effect status event types.
func WithEffectStatusTypes(types map[EffectStatusType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithPropertyTypes is a user input (uinput) device option to set the device
// property event types.
func WithPropertyTypes(types map[PropertyType]bool) UserInputOption {
	return func(u *UserInput) {
	}
}

// WithTypes is a user input (uinput) device option to set all types supported
// by the device at once.
func WithTypes(types ...interface{}) UserInputOption {
	return func(u *UserInput) {
		for _, typ := range types {
			switch typs := typ.(type) {
			case map[EventType]bool:
				WithEventTypes(typs)(u)
			case map[SyncType]bool:
				WithSyncTypes(typs)(u)
			case map[KeyType]bool:
				WithKeyTypes(typs)(u)
			case map[RelativeType]bool:
				WithRelativeTypes(typs)(u)
			case map[AbsoluteType]bool:
				WithAbsoluteTypes(typs)(u)
			case map[MiscType]bool:
				WithMiscTypes(typs)(u)
			case map[SwitchType]bool:
				WithSwitchTypes(typs)(u)
			case map[LEDType]bool:
				WithLEDTypes(typs)(u)
			case map[RepeatType]bool:
				WithRepeatTypes(typs)(u)
			case map[EffectType]bool:
				WithEffectTypes(typs)(u)
			case map[PowerType]bool:
				WithPowerTypes(typs)(u)
			case map[EffectStatusType]bool:
				WithEffectStatusTypes(typs)(u)
			case map[PropertyType]bool:
				WithPropertyTypes(typs)(u)
			default:
				panic(fmt.Sprintf("invalid type %T", typ))
			}
		}
	}
}

// WithTypesFromDev is a user input (uinput) device option to copy the options
// from a input event device.
func WithTypesFromDev(d *Evdev) UserInputOption {
	return func(u *UserInput) {
		WithTypes(
			d.EventTypes(),
			d.SyncTypes(),
			d.KeyTypes(),
			d.RelativeTypes(),
			d.AbsoluteTypes(),
			d.MiscTypes(),
			d.SwitchTypes(),
			d.LEDTypes(),
			//d.RepeatTypes(),
			d.SoundTypes(),
			d.EffectTypes(),
			d.PowerTypes(),
			d.EffectStatusTypes(),
		)(u)
	}
}

// uisetup wraps setup data sent to the UI_DEV_SETUP ioctl.
type uisetup struct {
	id         ID
	name       [80]byte
	effectsMax uint32
}
