package rexec

// A ProcessConfig describes how to manage a system process.
type ProcessConfig struct {
	Name    string   `json:"name"`
	Args    []string `json:"args"`
	CWD     string   `json:"cwd"`
	OneShot bool     `json:"one_shot"`
	Log     bool     `json:"log"`
}
