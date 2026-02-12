//go:build !no_cgo

package web

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-graphviz"
)

func (svc *webService) handleVisualizeResourceGraph(w http.ResponseWriter, r *http.Request) {
	const lookupParam = "history"
	redirectToLatestSnapshot := func() {
		url := *r.URL
		q := r.URL.Query()
		q.Del(lookupParam)
		url.RawQuery = q.Encode()

		http.Redirect(w, r, url.String(), http.StatusSeeOther)
	}

	lookupRawValue := strings.TrimSpace(r.URL.Query().Get(lookupParam))
	var (
		lookup int
		err    error
	)
	switch {
	case lookupRawValue == "":
		lookup = 0
	case lookupRawValue == "0":
		redirectToLatestSnapshot()
		return
	default:
		lookup, err = strconv.Atoi(lookupRawValue)
		if err != nil {
			redirectToLatestSnapshot()
			return
		}
	}

	snapshot, err := svc.r.ExportResourcesAsDot(lookup)
	if snapshot.Count == 0 {
		return
	}
	if err != nil {
		redirectToLatestSnapshot()
		return
	}

	write := func(s string) {
		//nolint:errcheck
		_, _ = w.Write([]byte(s))
	}

	layout := r.URL.Query().Get("layout")
	if layout == "text" {
		write(snapshot.Snapshot.Dot)
		return
	}

	gv := graphviz.New()
	defer func() {
		closeErr := gv.Close()
		if closeErr != nil {
			svc.r.Logger().Warn("failed to close graph visualizer")
		}
	}()

	graph, err := graphviz.ParseBytes([]byte(snapshot.Snapshot.Dot))
	if err != nil {
		return
	}
	if layout != "" {
		gv.SetLayout(graphviz.Layout(layout))
	}

	navButton := func(index int, label string) {
		url := *r.URL
		q := r.URL.Query()
		q.Set(lookupParam, strconv.Itoa(index))
		url.RawQuery = q.Encode()
		var html string
		if index < 0 || index >= snapshot.Count || index == lookup {
			html = fmt.Sprintf(`<a>%s</a>`, label)
		} else {
			html = fmt.Sprintf(`<a href=%q>%s</a>`, url.String(), label)
		}
		write(html)
	}

	// Navigation buttons
	write(`<html><div>`)
	navButton(0, "Latest")
	write(`|`)
	navButton(snapshot.Index-1, "Later")
	// Index counts from 0, but we want to show pages starting from 1
	write(fmt.Sprintf(`| %d / %d |`, snapshot.Index+1, snapshot.Count))
	navButton(snapshot.Index+1, "Earlier")
	write(`|`)
	navButton(snapshot.Count-1, "Earliest")
	write(`</div>`)

	// Snapshot capture timestamp
	write(fmt.Sprintf("<p>%s</p>", snapshot.Snapshot.CreatedAt.Format(time.UnixDate)))

	// HACK: We create a custom writer that removes the first 6 lines of XML written by
	// `gv.Render` - we exclude these lines of XML since they prevent us from adding HTML
	// elements to the rendered HTML. We depend on `gv.Render` calling fxml.Write exactly
	// one time.
	//
	// TODO(RSDK-6797): Parse the html text returned by `gv.Render` using an HTML parser
	// (https://pkg.go.dev/golang.org/x/net/html or equivalent) and remove the nodes that
	// prevent us from adding additional HTML.
	fxml := &filterXML{w: w}
	if err = gv.Render(graph, graphviz.SVG, fxml); err != nil {
		return
	}
	write(`</html>`)
}
