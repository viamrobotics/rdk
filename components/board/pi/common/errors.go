package picommon

import "github.com/pkg/errors"

// PiGPIOErrorMap maps the error codes to the human readable error names. This can be found at the pigpio C interface.
var PiGPIOErrorMap = map[int]string{
	-1:   "PI_INIT_FAILED: gpioInitialise failed",
	-2:   "PI_BAD_USER_GPIO: GPIO not 0-31",
	-3:   "PI_BAD_GPIO: GPIO not 0-53",
	-4:   "PI_BAD_MODE: mode not 0-7",
	-5:   "PI_BAD_LEVEL: level not 0-1",
	-6:   "PI_BAD_PUD: pud not 0-2",
	-7:   "PI_BAD_PULSEWIDTH: pulsewidth not 0 or 500-2500",
	-8:   "PI_BAD_DUTYCYCLE: dutycycle outside set range",
	-9:   "PI_BAD_TIMER: timer not 0-9",
	-10:  "PI_BAD_MS: ms not 10-60000",
	-11:  "PI_BAD_TIMETYPE: timetype not 0-1",
	-12:  "PI_BAD_SECONDS: seconds < 0",
	-13:  "PI_BAD_MICROS: micros not 0-999999",
	-14:  "PI_TIMER_FAILED: gpioSetTimerFunc failed",
	-15:  "PI_BAD_WDOG_TIMEOUT: timeout not 0-60000",
	-16:  "PI_NO_ALERT_FUNC: DEPRECATED",
	-17:  "PI_BAD_CLK_PERIPH: clock peripheral not 0-1",
	-18:  "PI_BAD_CLK_SOURCE: DEPRECATED",
	-19:  "PI_BAD_CLK_MICROS: clock micros not 1, 2, 4, 5, 8, or 10",
	-20:  "PI_BAD_BUF_MILLIS: buf millis not 100-10000",
	-21:  "PI_BAD_DUTYRANGE: dutycycle range not 25-40000",
	-22:  "PI_BAD_SIGNUM: signum not 0-63",
	-23:  "PI_BAD_PATHNAME: can't open pathname",
	-24:  "PI_NO_HANDLE: no handle available",
	-25:  "PI_BAD_HANDLE: unknown handle",
	-26:  "PI_BAD_IF_FLAGS: ifFlags > 4",
	-27:  "PI_BAD_CHANNEL_OR_PI_BAD_PRIM_CHANNEL: DMA channel not 0-15 OR DMA primary channel not 0-15",
	-28:  "PI_BAD_SOCKET_PORT: socket port not 1024-32000",
	-29:  "PI_BAD_FIFO_COMMAND: unrecognized fifo command",
	-30:  "PI_BAD_SECO_CHANNEL: DMA secondary channel not 0-15",
	-31:  "PI_NOT_INITIALISED: function called before gpioInitialise",
	-32:  "PI_INITIALISED: function called after gpioInitialise",
	-33:  "PI_BAD_WAVE_MODE: waveform mode not 0-3",
	-34:  "PI_BAD_CFG_INTERNAL: bad parameter in gpioCfgInternals call",
	-35:  "PI_BAD_WAVE_BAUD: baud rate not 50-250K(RX)/50-1M(TX)",
	-36:  "PI_TOO_MANY_PULSES: waveform has too many pulses",
	-37:  "PI_TOO_MANY_CHARS: waveform has too many chars",
	-38:  "PI_NOT_SERIAL_GPIO: no bit bang serial read on GPIO",
	-39:  "PI_BAD_SERIAL_STRUC: bad (null) serial structure parameter",
	-40:  "PI_BAD_SERIAL_BUF: bad (null) serial buf parameter",
	-41:  "PI_NOT_PERMITTED: GPIO operation not permitted",
	-42:  "PI_SOME_PERMITTED: one or more GPIO not permitted",
	-43:  "PI_BAD_WVSC_COMMND: bad WVSC subcommand",
	-44:  "PI_BAD_WVSM_COMMND: bad WVSM subcommand",
	-45:  "PI_BAD_WVSP_COMMND: bad WVSP subcommand",
	-46:  "PI_BAD_PULSELEN: trigger pulse length not 1-100",
	-47:  "PI_BAD_SCRIPT: invalid script",
	-48:  "PI_BAD_SCRIPT_ID: unknown script id",
	-49:  "PI_BAD_SER_OFFSET: add serial data offset > 30 minutes",
	-50:  "PI_GPIO_IN_USE: GPIO already in use",
	-51:  "PI_BAD_SERIAL_COUNT: must read at least a byte at a time",
	-52:  "PI_BAD_PARAM_NUM: script parameter id not 0-9",
	-53:  "PI_DUP_TAG: script has duplicate tag",
	-54:  "PI_TOO_MANY_TAGS: script has too many tags",
	-55:  "PI_BAD_SCRIPT_CMD: illegal script command",
	-56:  "PI_BAD_VAR_NUM: script variable id not 0-149",
	-57:  "PI_NO_SCRIPT_ROOM: no more room for scripts",
	-58:  "PI_NO_MEMORY: can't allocate temporary memory",
	-59:  "PI_SOCK_READ_FAILED: socket read failed",
	-60:  "PI_SOCK_WRIT_FAILED: socket write failed",
	-61:  "PI_TOO_MANY_PARAM: too many script parameters (> 10)",
	-62:  "PI_SCRIPT_NOT_READY: script initialising",
	-63:  "PI_BAD_TAG: script has unresolved tag",
	-64:  "PI_BAD_MICS_DELAY: bad MICS delay (too large)",
	-65:  "PI_BAD_MILS_DELAY: bad MILS delay (too large)",
	-66:  "PI_BAD_WAVE_ID: non existent wave id",
	-67:  "PI_TOO_MANY_CBS: No more CBs for waveform",
	-68:  "PI_TOO_MANY_OOL: No more OOL for waveform",
	-69:  "PI_EMPTY_WAVEFORM: attempt to create an empty waveform",
	-70:  "PI_NO_WAVEFORM_ID: no more waveforms",
	-71:  "PI_I2C_OPEN_FAILED: can't open I2C device",
	-72:  "PI_SER_OPEN_FAILED: can't open serial device",
	-73:  "PI_SPI_OPEN_FAILED: can't open SPI device",
	-74:  "PI_BAD_I2C_BUS: bad I2C bus",
	-75:  "PI_BAD_I2C_ADDR: bad I2C address",
	-76:  "PI_BAD_SPI_CHANNEL: bad SPI channel",
	-77:  "PI_BAD_FLAGS: bad i2c/spi/ser open flags",
	-78:  "PI_BAD_SPI_SPEED: bad SPI speed",
	-79:  "PI_BAD_SER_DEVICE: bad serial device name",
	-80:  "PI_BAD_SER_SPEED: bad serial baud rate",
	-81:  "PI_BAD_PARAM: bad i2c/spi/ser parameter",
	-82:  "PI_I2C_WRITE_FAILED: i2c write failed",
	-83:  "PI_I2C_READ_FAILED: i2c read failed",
	-84:  "PI_BAD_SPI_COUNT: bad SPI count",
	-85:  "PI_SER_WRITE_FAILED: ser write failed",
	-86:  "PI_SER_READ_FAILED: ser read failed",
	-87:  "PI_SER_READ_NO_DATA: ser read no data available",
	-88:  "PI_UNKNOWN_COMMAND: unknown command",
	-89:  "PI_SPI_XFER_FAILED: spi xfer/read/write failed",
	-90:  "PI_BAD_POINTER: bad (NULL) pointer",
	-91:  "PI_NO_AUX_SPI: no auxiliary SPI on Pi A or B",
	-92:  "PI_NOT_PWM_GPIO: GPIO is not in use for PWM",
	-93:  "PI_NOT_SERVO_GPI: GPIO is not in use for servo pulses",
	-94:  "PI_NOT_HCLK_GPIO: GPIO has no hardware clock",
	-95:  "PI_NOT_HPWM_GPIO: GPIO has no hardware PWM",
	-96:  "PI_BAD_HPWM_FREQ: invalid hardware PWM frequency",
	-97:  "PI_BAD_HPWM_DUTY: hardware PWM dutycycle not 0-1M",
	-98:  "PI_BAD_HCLK_FREQ: invalid hardware clock frequency",
	-99:  "PI_BAD_HCLK_PASS: need password to use hardware clock 1",
	-100: "PI_HPWM_ILLEGAL: illegal, PWM in use for main clock",
	-101: "PI_BAD_DATABITS: serial data bits not 1-32",
	-102: "PI_BAD_STOPBITS: serial (half) stop bits not 2-8",
	-103: "PI_MSG_TOOBIG: socket/pipe message too big",
	-104: "PI_BAD_MALLOC_MODE: bad memory allocation mode",
	-105: "PI_TOO_MANY_SEGS: too many I2C transaction segments",
	-106: "PI_BAD_I2C_SEG: an I2C transaction segment failed",
	-107: "PI_BAD_SMBUS_CMD: SMBus command not supported by driver",
	-108: "PI_NOT_I2C_GPIO: no bit bang I2C in progress on GPIO",
	-109: "PI_BAD_I2C_WLEN: bad I2C write length",
	-110: "PI_BAD_I2C_RLEN: bad I2C read length",
	-111: "PI_BAD_I2C_CMD: bad I2C command",
	-112: "PI_BAD_I2C_BAUD: bad I2C baud rate, not 50-500k",
	-113: "PI_CHAIN_LOOP_CNT: bad chain loop count",
	-114: "PI_BAD_CHAIN_LOOP: empty chain loop",
	-115: "PI_CHAIN_COUNTER: too many chain counters",
	-116: "PI_BAD_CHAIN_CMD: bad chain command",
	-117: "PI_BAD_CHAIN_DELAY: bad chain delay micros",
	-118: "PI_CHAIN_NESTING: chain counters nested too deeply",
	-119: "PI_CHAIN_TOO_BIG: chain is too long",
	-120: "PI_DEPRECATED: deprecated function removed",
	-121: "PI_BAD_SER_INVERT: bit bang serial invert not 0 or 1",
	-122: "PI_BAD_EDGE: bad ISR edge value, not 0-2",
	-123: "PI_BAD_ISR_INIT: bad ISR initialisation",
	-124: "PI_BAD_FOREVER: loop forever must be last command",
	-125: "PI_BAD_FILTER: bad filter parameter",
	-126: "PI_BAD_PAD: bad pad number",
	-127: "PI_BAD_STRENGTH: bad pad drive strength",
	-128: "PI_FIL_OPEN_FAILED: file open failed",
	-129: "PI_BAD_FILE_MODE: bad file mode",
	-130: "PI_BAD_FILE_FLAG: bad file flag",
	-131: "PI_BAD_FILE_READ: bad file read",
	-132: "PI_BAD_FILE_WRITE: bad file write",
	-133: "PI_FILE_NOT_ROPEN: file not open for read",
	-134: "PI_FILE_NOT_WOPEN: file not open for write",
	-135: "PI_BAD_FILE_SEEK: bad file seek",
	-136: "PI_NO_FILE_MATCH: no files match pattern",
	-137: "PI_NO_FILE_ACCESS: no permission to access file",
	-138: "PI_FILE_IS_A_DIR: file is a directory",
	-139: "PI_BAD_SHELL_STATUS: bad shell return status",
	-140: "PI_BAD_SCRIPT_NAME: bad script name",
	-141: "PI_BAD_SPI_BAUD: bad SPI baud rate, not 50-500k",
	-142: "PI_NOT_SPI_GPIO: no bit bang SPI in progress on GPIO",
	-143: "PI_BAD_EVENT_ID: bad event id",
	-144: "PI_CMD_INTERRUPTED: Used by Python",
	-145: "PI_NOT_ON_BCM2711: not available on BCM2711",
	-146: "PI_ONLY_ON_BCM2711: only available on BCM271",
}

// given message and error code returns a human-readable string
func ConvertErrorCodeToMessage(errorCode int, message string) error {
	errorMessage, exists := PiGPIOErrorMap[errorCode]
	if exists {
		return errors.Errorf("%s: %s", message, errorMessage)
	}
	return errors.Errorf("%s: %d", message, errorCode)
}
