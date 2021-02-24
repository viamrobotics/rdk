package board

type MotorConfig struct {
	Name string
	Pins map[string]string
}

type AnalogConfig struct {
	Name string
	Pin  string
}

type Config struct {
	Name    string
	Model   string // example: "pi"
	Motors  []MotorConfig
	Analogs []AnalogConfig
}
