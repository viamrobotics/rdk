package web

type Options struct {
	AutoTile bool
	Pprof    bool
}

func NewOptions() Options {
	return Options{
		AutoTile: true,
		Pprof:    false,
	}
}
