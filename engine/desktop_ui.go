package main

import (
	_ "embed"
	"net/http"
)

//go:embed noir_ui.html
var noirEmbeddedUI []byte

//go:embed nocturne_icon.png
var nocturneEmbeddedIcon []byte

func (e *Engine) handleNoirUI(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(noirEmbeddedUI)
}

func (e *Engine) handleNocturneIcon(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(nocturneEmbeddedIcon)
}
