package main

import (
	"context"
	"fmt"
	stdhttp "net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/intentos/backend/internal/config"
	"github.com/intentos/backend/internal/database"
	apphttp "github.com/intentos/backend/internal/http"
)

func main() {
	// Load .env if present
	_ = godotenv.Load()

	// Setup logger
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	// Load config
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to load config")
	}

	// Database connection pool
	dbPool, err := pgxpool.New(context.Background(), cfg.DatabaseURL())
	if err != nil {
		log.Fatal().Err(err).Msg("cannot connect to database")
	}
	defer dbPool.Close()

	// Initialize DB schema (run migrations)
	if err := database.RunMigrations(context.Background(), dbPool); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}

	// Router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// CORS middleware for Vercel / cross-origin deployments
	r.Use(func(next stdhttp.Handler) stdhttp.Handler {
		return stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
			} else {
				w.Header().Set("Access-Control-Allow-Origin", "*")
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Accept, Authorization, Content-Type, X-CSRF-Token")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			if r.Method == "OPTIONS" {
				w.WriteHeader(stdhttp.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})

	// Register routes
	apphttp.RegisterRoutes(r, dbPool, cfg)

	srv := &stdhttp.Server{Addr: fmt.Sprintf(":%s", cfg.Port), Handler: r}

	// Graceful shutdown
	go func() {
		log.Info().Msgf("server listening on %s", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != stdhttp.ErrServerClosed {
			log.Fatal().Err(err).Msg("server error")
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Info().Msg("shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal().Err(err).Msg("server forced to shutdown")
	}
	log.Info().Msg("server exited")
}
