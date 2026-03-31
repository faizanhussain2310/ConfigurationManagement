package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/faizanhussain/arbiter/pkg/store"
	"github.com/go-chi/chi/v5"
)

// NewRouter creates the Chi router with all API routes and SPA fallback.
func NewRouter(s *store.Store, webFS fs.FS) http.Handler {
	h := &Handler{Store: s}
	r := chi.NewRouter()

	// Global middleware
	r.Use(Logger)
	r.Use(CORS)
	r.Use(MaxBodySize)

	// Health check
	r.Get("/api/health", h.HealthCheck)

	// IMPORTANT: register static routes before parameterized routes.
	// Chi matches routes in order. /api/rules/import must come before
	// /api/rules/{id} or "import" gets treated as an ID.
	r.Post("/api/rules/import", h.ImportRule)

	// Batch evaluate (static path, must be before /api/rules/{id})
	r.Post("/api/evaluate", h.BatchEvaluate)

	// Rules CRUD
	r.Post("/api/rules", h.CreateRule)
	r.Get("/api/rules", h.ListRules)

	r.Route("/api/rules/{id}", func(r chi.Router) {
		r.Get("/", h.GetRule)
		r.Put("/", h.UpdateRule)
		r.Delete("/", h.DeleteRule)
		r.Post("/evaluate", h.EvaluateRule)
		r.Get("/history", h.GetEvalHistory)
		r.Get("/versions", h.ListVersions)
		r.Post("/rollback/{version}", h.RollbackToVersion)
		r.Post("/duplicate", h.DuplicateRule)
		r.Get("/export", h.ExportRule)
	})

	// SPA fallback: serve static files from embedded web/dist,
	// and return index.html for any non-API, non-static path.
	if webFS != nil {
		fileServer := http.FileServer(http.FS(webFS))

		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// Don't serve SPA for API routes (shouldn't reach here, but safety)
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}

			// Try to serve the file directly
			path := strings.TrimPrefix(r.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			// Check if file exists in the embedded FS
			f, err := webFS.Open(path)
			if err == nil {
				f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}

			// File not found: serve index.html (SPA fallback)
			indexFile, err := webFS.Open("index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			defer indexFile.Close()

			stat, err := indexFile.Stat()
			if err != nil {
				http.NotFound(w, r)
				return
			}

			http.ServeContent(w, r, "index.html", stat.ModTime(), indexFile.(readSeeker))
		})
	}

	return r
}

// readSeeker combines io.ReadSeeker for http.ServeContent.
type readSeeker interface {
	Read(p []byte) (n int, err error)
	Seek(offset int64, whence int) (int64, error)
}
