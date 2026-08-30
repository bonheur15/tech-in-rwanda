// Package app composes the API features and production static-site delivery.
package app

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"rwandafreespace.com/blog/backend/internal/features/servertime"
	"rwandafreespace.com/blog/backend/internal/platform/config"
	"rwandafreespace.com/blog/backend/internal/platform/httpx"
)

func New(cfg config.Config, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()
	mux.Handle(servertime.Route, servertime.NewHandler(time.Now))
	mux.HandleFunc("/api/v1/healthz", health)
	mux.Handle("/", staticSite(cfg.StaticDir))

	return httpx.Chain(
		mux,
		httpx.RequestID,
		httpx.SecurityHeaders,
		httpx.CORS(cfg.AllowedOrigins),
		httpx.Recover(logger),
		httpx.Log(logger),
	)
}

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func staticSite(directory string) http.Handler {
	if directory == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
	}

	root := http.Dir(directory)
	files := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := path.Clean("/" + r.URL.Path)
		name := strings.TrimPrefix(cleanPath, "/")

		file, err := root.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		info, statErr := file.Stat()
		_ = file.Close()
		if statErr != nil {
			http.NotFound(w, r)
			return
		}
		if info.IsDir() {
			index, indexErr := root.Open(path.Join(name, "index.html"))
			if indexErr != nil {
				http.NotFound(w, r)
				return
			}
			indexInfo, indexStatErr := index.Stat()
			_ = index.Close()
			if indexStatErr != nil || indexInfo.IsDir() {
				http.NotFound(w, r)
				return
			}
		}

		if strings.HasPrefix(r.URL.Path, "/_astro/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			w.Header().Set("Cache-Control", "no-cache")
		}
		files.ServeHTTP(w, r)
	})
}

func NewServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.Address,
		Handler:           handler,
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}
}

func StaticDirectoryExists(directory string) bool {
	if directory == "" {
		return false
	}
	info, err := os.Stat(directory)
	return err == nil && info.IsDir()
}
