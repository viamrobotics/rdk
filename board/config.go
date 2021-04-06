package board

type MotorConfig struct {
	Name             string
	Pins             map[string]string
	Encoder          string // name of the digital interrupt that is the encoder
	EncoderB         string // name of the digital interrupt that is hall encoder b
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
	Name    string
	Pin     string
	Type    string // e.g. basic, servo
	Formula string
}

type Config struct {
	Name              string
	Model             string // example: "pi"
	Motors            []MotorConfig
	Servos            []ServoConfig
	Analogs           []AnalogConfig
	DigitalInterrupts []DigitalInterruptConfig
}
