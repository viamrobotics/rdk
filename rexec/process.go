package rexec

type ProcessConfig struct {
	Name    string   `json:"name"`
	Args    []string `json:"args"`
	OneShot bool     `json:"one_shot"`
	Log     bool     `json:"log"`
}
