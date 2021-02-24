package board

type MotorConfig struct {
	Name string
	Pins map[string]string
}

type AnalogConfig struct {
	Name string
	Pin  string
}

type DigitalInterruptConfig struct {
	Name string
	Pin  string
	Mode string // falling, rising
}

type Config struct {
	Name              string
	Model             string // example: "pi"
	Motors            []MotorConfig
	Analogs           []AnalogConfig
	DigitalInterrupts []DigitalInterruptConfig
}
