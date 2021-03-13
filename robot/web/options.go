package web

type Options struct {
	AutoTile bool
}

func NewOptions() Options {
	return Options{
		AutoTile: true,
	}
}
