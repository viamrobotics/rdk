package rexec

type ProcessConfig struct {
	Name    string   `json:"name"`
	Args    []string `json:"args"`
	CWD     string   `json:"cwd"`
	OneShot bool     `json:"one_shot"`
	Log     bool     `json:"log"`
}
