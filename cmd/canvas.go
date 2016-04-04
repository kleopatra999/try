package main

import (
	"net/http"
	"path"

	"github.com/tile38/play/assets"
)

func canvas(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[1:]
	if name == "" {
		name = "index.html"
	}
	data, err := assets.Asset(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch path.Ext(name) {
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	}
	w.Write(data)
}
