// Package server wires the API and the embedded SPA into one HTTP handler and
// manages the listener lifecycle with graceful shutdown.
package server

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"goscp/internal/api"
	"goscp/internal/assets"
	"goscp/internal/config"
	"goscp/internal/storage"
)

// Server owns the http.Server and its dependencies.
type Server struct {
	cfg  *config.Config
	http *http.Server
}

// New builds a fully wired Server.
func New(cfg *config.Config) (*Server, error) {
	store := storage.New(cfg.Root)
	a := api.New(store, cfg.MaxUploadBytes, cfg.Token, cfg.Password)

	mux := http.NewServeMux()
	a.Routes(mux)

	// SPA / static asset handler for everything not under /api.
	spa, err := newSPAHandler()
	if err != nil {
		return nil, err
	}
	mux.Handle("/", spa)

	// API routes already carry their own auth via the chained middleware below.
	handler := api.Chain(mux,
		api.Logger,
		api.CORS,
		tokenGuard(cfg.Token),
	)

	return &Server{
		cfg: cfg,
		http: &http.Server{
			Addr:              cfg.Addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
	}, nil
}

// tokenGuard applies bearer-token auth to /api routes only; static assets and
// the SPA shell stay public so the login screen itself can load.
func tokenGuard(token string) api.Middleware {
	guarded := api.RequireToken(token)
	return func(next http.Handler) http.Handler {
		protected := guarded(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// The token-issuing endpoint must stay public — it is how a client
			// obtains the bearer token in the first place; it enforces its own
			// password check.
			public := r.URL.Path == "/api/v1/token"
			if strings.HasPrefix(r.URL.Path, "/api/") && r.Method != http.MethodOptions && !public {
				protected.ServeHTTP(w, r)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ListenAndServe starts the server (blocking).
func (s *Server) ListenAndServe() error {
	return s.http.ListenAndServe()
}

// Shutdown gracefully drains in-flight requests.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

// newSPAHandler serves the embedded frontend, falling back to index.html for
// client-side routes. If no frontend is embedded it returns a helpful notice.
func newSPAHandler() (http.Handler, error) {
	if !assets.HasIndex() {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("goSCP API is running.\nFrontend not embedded — run `make web` then rebuild.\n"))
		}), nil
	}

	sub, err := assets.FS()
	if err != nil {
		return nil, err
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := path.Clean(strings.TrimPrefix(r.URL.Path, "/"))
		if clean == "." {
			clean = "index.html"
		}
		if _, err := fs.Stat(sub, clean); err != nil {
			if os.IsNotExist(err) {
				// SPA fallback: serve index.html for client-side routes.
				r2 := r.Clone(r.Context())
				r2.URL.Path = "/"
				fileServer.ServeHTTP(w, r2)
				return
			}
		}
		fileServer.ServeHTTP(w, r)
	}), nil
}
