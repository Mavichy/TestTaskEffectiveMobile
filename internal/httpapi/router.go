package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/Mavichy/TestTaskEffectiveMobile/internal/storage"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewServer(addr string, logger *slog.Logger, repo *storage.SubscriptionsRepo) *http.Server {
	h := NewHandler(logger, repo)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(15 * time.Second))
	r.Use(requestLogger(logger))

	r.Route("/subscriptions", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/", h.List)
		r.Get("/total", h.Total)

		r.Get("/{id}", h.Get)
		r.Patch("/{id}", h.Update)
		r.Delete("/{id}", h.Delete)

	})

	return &http.Server{
		Addr:    addr,
		Handler: r,
	}
}

func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			next.ServeHTTP(w, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"query", r.URL.RawQuery,
				"ms", time.Since(start).Milliseconds(),
			)
		})
	}
}
