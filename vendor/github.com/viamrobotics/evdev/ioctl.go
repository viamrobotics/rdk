package evdev

import (
	"bytes"
	"fmt"
	"syscall"
	"unsafe"
)

// most of the funcs, consts, and vars defined here are rewritten macros or
// #defines taken from Linux's ioctl.h, input.h, and uinput.h.

const (
	iocNone     = 0x0
	iocWrite    = 0x1
	iocRead     = 0x2
	iocNrBits   = 8
	iocTypeBits = 8
	iocSizeBits = 14
	//iocDirBits  = 2
	iocNrShift = 0
	//iocNrMask   = (1 << iocNrBits) - 1
	//iocTypeMask  = (1 << iocTypeBits) - 1
	//iocSizeMask = (1 << iocSizeBits) - 1
	//iocDirMask   = (1 << iocDirBits) - 1
	iocTypeShift = iocNrShift + iocNrBits
	iocSizeShift = iocTypeShift + iocTypeBits
	iocDirShift  = iocSizeShift + iocSizeBits
	//iocIn        = iocWrite << iocDirShift
	//iocOut       = iocRead << iocDirShift
	//iocInOut     = (iocWrite | iocRead) << iocDirShift
	//iocSize_mask = iocSizeMask << iocSizeShift
)

func ioc(dir, t, nr, size int) uintptr {
	return uintptr((dir << iocDirShift) | (t << iocTypeShift) |
		(nr << iocNrShift) | (size << iocSizeShift))
}

func io(t, nr int) uintptr {
	return ioc(iocNone, t, nr, 0)
}

func ior(t, nr, size int) uintptr {
	return ioc(iocRead, t, nr, size)
}

func iow(t, nr, size int) uintptr {
	return ioc(iocWrite, t, nr, size)
}

/*func iowr(t, nr, size int) uintptr {
	return ioc(iocRead|iocWrite, t, nr, size)
}*/

/*func iocDir(nr int) uintptr {
	return uintptr(((nr) >> iocDirShift) & iocDirMask)
}*/

/*func iocType(nr int) uintptr {
	return uintptr(((nr) >> iocTypeShift) & iocTypeMask)
}*/

/*func iocNr(nr int) uintptr {
	return uintptr(((nr) >> iocNrShift) & iocNrMask)
}*/

/*func iocSize(nr int) uintptr {
	return uintptr(((nr) >> iocSizeShift) & iocSizeMask)
}*/

// input event and uinput ioctl bases.
const (
	evBase = 'E'
	uiBase = 'U'
)

var (
	// sizes
	sizeofEvent int
	sizeofAxis  int

	// input event ioctl
	evGetVersion   uintptr
	evGetID        uintptr
	evGetRep       uintptr
	evSetRep       uintptr
	evGetKeycode   uintptr
	evGetKeycodeV2 uintptr
	evSetKeycode   uintptr
	evSetKeycodeV2 uintptr
	evSetFF        uintptr
	evDelFF        uintptr
	evGetEffects   uintptr
	evGrab         uintptr
	//evSetClockID   uintptr

	// uinput ioctl
	uiDevCreate  uintptr
	uiDevDestroy uintptr
	uiDevSetup   uintptr
	uiSetPhys    uintptr
)

func init() {
	sizeofEvent = int(unsafe.Sizeof(Event{}))
	sizeofAxis = int(unsafe.Sizeof(Axis{}))

	sizeofInt := int(unsafe.Sizeof(int32(0)))
	sizeofInt2 := sizeofInt << 1
	sizeofKeymap := int(unsafe.Sizeof(KeyMap{}))

	// input event
	evGetVersion = ior(evBase, 0x01, sizeofInt)
	evGetID = ior(evBase, 0x02, int(unsafe.Sizeof(ID{})))
	evGetRep = ior(evBase, 0x03, sizeofInt2)
	evSetRep = iow(evBase, 0x03, sizeofInt2)
	evGetKeycode = ior(evBase, 0x04, sizeofInt2)
	evGetKeycodeV2 = ior(evBase, 0x04, sizeofKeymap)
	evSetKeycode = iow(evBase, 0x04, sizeofInt2)
	evSetKeycodeV2 = iow(evBase, 0x04, sizeofKeymap)
	evSetFF = ioc(iocWrite, evBase, 0x80, int(unsafe.Sizeof(Effect{})))
	evDelFF = iow(evBase, 0x81, sizeofInt)
	evGetEffects = ior(evBase, 0x84, sizeofInt)
	evGrab = iow(evBase, 0x90, sizeofInt)
	//evSetClockID = iow(evBase, 0xa0, sizeofInt)

	// uinput
	uiDevCreate = io(uiBase, 1)
	uiDevDestroy = io(uiBase, 2)
	uiDevSetup = iow(uiBase, 3, int(unsafe.Sizeof(uisetup{})))
	uiSetPhys = iow(uiBase, 108, int(unsafe.Sizeof(uintptr(0))))
}

func evGetName(n int) uintptr {
	return ioc(iocRead, evBase, 0x06, n)
}

func evGetPhys(n int) uintptr {
	return ioc(iocRead, evBase, 0x07, n)
}

func evGetUniq(n int) uintptr {
	return ioc(iocRead, evBase, 0x08, n)
}

func evGetProp(n int) uintptr {
	return ioc(iocRead, evBase, 0x09, n)
}

func evGetMtSlots(n int) uintptr {
	return ioc(iocRead, evBase, 0x0a, n)
}

func evGetKey(n int) uintptr {
	return ioc(iocRead, evBase, 0x18, n)
}

func evGetLed(n int) uintptr {
	return ioc(iocRead, evBase, 0x19, n)
}

func evGetSnd(n int) uintptr {
	return ioc(iocRead, evBase, 0x1a, n)
}

func evGetSw(n int) uintptr {
	return ioc(iocRead, evBase, 0x1b, n)
}

func evGetBit(ev, n int) uintptr {
	return ioc(iocRead, evBase, 0x20+ev, n)
}

func evGetAbs(abs int) uintptr {
	return ior(evBase, 0x40+abs, sizeofAxis)
}

func evSetAbs(abs int) uintptr {
	return iow(evBase, 0xc0+abs, sizeofAxis)
}

func uiGetSysname(n int) uintptr {
	return ioc(iocRead, uiBase, 44, n)
}

// ioctl wraps sending an ioctl syscall.
func ioctl(fd, name uintptr, data interface{}) error {
	var v uintptr

	switch dd := data.(type) {
	case unsafe.Pointer:
		v = uintptr(dd)

	case int:
		v = uintptr(dd)

	case uintptr:
		v = dd

	default:
		return fmt.Errorf("ioctl: data has invalid type %T", data)
	}

	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, fd, name, v)
	if errno != 0 {
		return errno
	}
	return nil
}

// ioctlString wraps reading a string from an ioctl syscall.
func ioctlString(fd uintptr, f func(int) uintptr, n int) (string, error) {
	buf := make([]byte, n)
	err := ioctl(fd, f(n), unsafe.Pointer(&buf[0]))
	if err != nil {
		return "", err
	}
	if i := bytes.IndexByte(buf, 0); i != -1 {
		return string(buf[:i]), nil
	}
	return string(buf), nil
}
