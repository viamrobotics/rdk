package genericlinux

import "fmt"

// GPIOBoardMapping represents a GPIO pin's location locally within a GPIO chip
// and globally within sysfs.
type GPIOBoardMapping struct {
	GPIOChipDev    string
	GPIO           int
	GPIOName       string
	PWMSysFsDir    string // Absolute path to the directory, empty string for none
	PWMID          int
	HWPWMSupported bool
}

// PinDefinition describes board specific information on how a particular pin can be accessed
// via sysfs along with information about its PWM capabilities.
type PinDefinition struct {
	GPIOChipRelativeIDs map[int]int    // ngpio -> relative id
	GPIONames           map[int]string // e.g. ngpio=169=PQ.06 for claraAGXXavier
	GPIOChipSysFSDir    string
	PinNumberBoard      int
	PinNumberBCM        int
	PinNameCVM          string
	PinNameTegraSOC     string
	PWMChipSysFSDir     string // empty for none
	PWMID               int    // -1 for none
}

// board types have to be below so non-linux devices will compile

// BoardInformation details pin definitions and device compatibility for a particular board.
type BoardInformation struct {
	PinDefinitions []PinDefinition
	Compats        []string
}

// A NoBoardFoundError is returned when no compatible mapping is found for a board during GPIO board mapping.
type NoBoardFoundError struct {
	modelName string
}

func (err NoBoardFoundError) Error() string {
	return fmt.Sprintf("could not determine %q model", err.modelName)
}

func noBoardError(modelName string) error {
	return fmt.Errorf("could not determine %q model", modelName)
}

// gpioChipData is a struct used solely within GetGPIOBoardMappings and its sub-pieces. It
// describes a GPIO chip within sysfs.
type gpioChipData struct {
	Dir   string // Pseudofile within sysfs to interact with this chip
	Ngpio int    // Taken from the /ngpio pseudofile in sysfs: number of lines on the chip
}

// pwmChipData is a struct used solely within GetGPIOBoardMappings and its sub-pieces. It
// describes a PWM chip within sysfs.
type pwmChipData struct {
	Dir  string // Absolute path to pseudofile within sysfs to interact with this chip
	Npwm int    // Taken from the /npwm pseudofile in sysfs: number of lines on the chip
}
