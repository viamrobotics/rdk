package perf

import (
	"net/http"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/stats/view"
)

// See https://opencensus.io/guides/http/go/net_http/server/

func registerHTTPViews() error {
	if err := view.Register(ochttp.DefaultServerViews...); err != nil {
		return err
	}

	if err := view.Register(ochttp.DefaultClientViews...); err != nil {
		return err
	}

	return nil
}

// WrapHTTPHandlerForStats wraps a http handler with stats collection writing to opencensus.
//
// Example:
//
//	httpHandler := goji.NewMux()
//	hanlderWithStats = perf.WrapHTTPHandlerForStats(httpHandler)
func WrapHTTPHandlerForStats(h http.Handler) http.Handler {
	return &ochttp.Handler{
		Handler:          h,
		IsPublicEndpoint: true,
	}
}

// NewRoundTripperWithStats creates a new RoundTripper with stats collecting writing to opencensus.
//
// Example:
//
//	client := &http.Client{Transport: perf.NewRoundTripperWithStats()},
func NewRoundTripperWithStats() http.RoundTripper {
	return new(ochttp.Transport)
}
