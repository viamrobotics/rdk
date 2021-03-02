package board

type MotorConfig struct {
	Name             string
	Pins             map[string]string
	Encoder          string // name of the digital interrupt that is the encoder
	TicksPerRotation int
}

type ServoConfig struct {
	Name string
	Pin  string
}

type AnalogConfig struct {
	Name              string
	Pin               string
	AverageOverMillis int
	SamplesPerSecond  int
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
	Servos            []ServoConfig
	Analogs           []AnalogConfig
	DigitalInterrupts []DigitalInterruptConfig
}
