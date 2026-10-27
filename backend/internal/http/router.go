package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/intentos/backend/internal/auth"
	"github.com/intentos/backend/internal/config"
	"github.com/intentos/backend/internal/intentions"
	"github.com/intentos/backend/internal/planner"
	"github.com/intentos/backend/internal/tasks"
)

// RegisterRoutes registers HTTP routes with chi router at both root and /api prefixes.
func RegisterRoutes(r *chi.Mux, db *pgxpool.Pool, cfg *config.Config) {
	// Register on root router (e.g. /health, /auth/login)
	mountEndpoints(r, db, cfg)

	// Also mount on /api (e.g. /api/health, /api/auth/login) for direct cross-origin frontends
	r.Route("/api", func(apiRouter chi.Router) {
		mountEndpoints(apiRouter, db, cfg)
	})
}

func mountEndpoints(r chi.Router, db *pgxpool.Pool, cfg *config.Config) {
	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Auth endpoints (no auth required)
	r.Post("/auth/register", auth.RegisterHandler(db, cfg))
	r.Post("/auth/login", auth.LoginHandler(db, cfg))

	// Protected routes
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(cfg.JWTSecret))

		// Intentions
		r.Post("/intentions", intentions.CreateIntentionHandler(db))
		r.Get("/intentions", intentions.ListIntentionsHandler(db))
		r.Get("/intentions/{id}", intentions.GetIntentionHandler(db))
		r.Put("/intentions/{id}", intentions.UpdateIntentionHandler(db))
		r.Delete("/intentions/{id}", intentions.DeleteIntentionHandler(db))

		// Tasks
		r.Post("/tasks", tasks.CreateTaskHandler(db))
		r.Get("/tasks", tasks.ListTasksHandler(db))
		r.Get("/tasks/{id}", tasks.GetTaskHandler(db))
		r.Put("/tasks/{id}", tasks.UpdateTaskHandler(db))
		r.Delete("/tasks/{id}", tasks.DeleteTaskHandler(db))
		r.Post("/tasks/{id}/dependencies", tasks.AddDependencyHandler(db))
		r.Delete("/tasks/{id}/dependencies/{depId}", tasks.RemoveDependencyHandler(db))

		// Planner
		r.Post("/plan/generate", planner.GeneratePlanHandler(db))
		r.Post("/plan/replan", planner.ReplanHandler(db))
		r.Post("/plan/reconcile", planner.ReconcileHandler(db))
		r.Get("/plan/blocks", planner.GetBlocksHandler(db))
		r.Get("/plan/history", planner.GetHistoryHandler(db))
	})
}
