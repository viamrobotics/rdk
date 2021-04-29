package web

type Options struct {
	AutoTile  bool
	Pprof     bool
	Port      int
	SharedDir string
}

func NewOptions() Options {
	return Options{
		AutoTile: true,
		Pprof:    false,
		Port:     8080,
	}
}
