package webapp

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"tialloystudio/internal/app"
	"tialloystudio/internal/httpapi"
)

//go:embed static/*
var assets embed.FS

func New(state *app.State) http.Handler {
	api := httpapi.NewHandler(state)
	sub, _ := fs.Sub(assets, "static")
	files := http.FileServer(http.FS(sub))
	index, _ := fs.ReadFile(sub, "index.html")
	// Keep project/provenance controls in a small independent JS module so the
	// validated scientific viewer remains isolated from project-management UI.
	page := strings.Replace(string(index), "</body>", "<script src=\"/project.js\"></script></body>", 1)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(page))
			return
		}
		files.ServeHTTP(w, r)
	})
}
