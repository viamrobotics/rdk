//go:build no_cgo

package web

import "net/http"

// stub for missing graphviz
func (svc *webService) handleVisualizeResourceGraph(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(`<html><body>Resource graph not supported on this build</body></html>`))
}
