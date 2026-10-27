package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/bcrypt"

	"github.com/intentos/backend/internal/config"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type userInfo struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type authResponse struct {
	Token string   `json:"token"`
	User  userInfo `json:"user"`
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func RegisterHandler(db *pgxpool.Pool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req registerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request payload", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.Password = strings.TrimSpace(req.Password)

		if req.Email == "" || req.Password == "" {
			writeJSONError(w, "email and password are required", http.StatusBadRequest)
			return
		}

		if len(req.Password) < 6 {
			writeJSONError(w, "password must be at least 6 characters", http.StatusBadRequest)
			return
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			log.Error().Err(err).Msg("hashing password failed")
			writeJSONError(w, "internal server error", http.StatusInternalServerError)
			return
		}

		newID := uuid.New()
		now := time.Now().UTC()
		_, err = db.Exec(r.Context(),
			"INSERT INTO users (id, email, password_hash, created_at, updated_at) VALUES ($1,$2,$3,$4,$5)",
			newID, req.Email, string(hash), now, now)

		if err != nil {
			if pgErr, ok := err.(*pgconn.PgError); ok && pgErr.Code == "23505" {
				writeJSONError(w, "an account with this email already exists", http.StatusConflict)
				return
			}
			log.Error().Err(err).Msg("insert user failed")
			writeJSONError(w, "could not create user", http.StatusInternalServerError)
			return
		}

		// Generate token so registration can authenticate immediately
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": newID.String(),
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		})
		signed, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			log.Error().Err(err).Msg("signing token failed")
			writeJSONError(w, "could not generate authentication token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(authResponse{
			Token: signed,
			User: userInfo{
				ID:    newID.String(),
				Email: req.Email,
			},
		})
	}
}

func LoginHandler(db *pgxpool.Pool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request payload", http.StatusBadRequest)
			return
		}

		req.Email = strings.TrimSpace(strings.ToLower(req.Email))
		req.Password = strings.TrimSpace(req.Password)

		var storedHash string
		var userID uuid.UUID
		err := db.QueryRow(r.Context(), "SELECT id, password_hash FROM users WHERE email=$1", req.Email).Scan(&userID, &storedHash)
		if err != nil {
			writeJSONError(w, "invalid email or password", http.StatusUnauthorized)
			return
		}

		if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.Password)); err != nil {
			writeJSONError(w, "invalid email or password", http.StatusUnauthorized)
			return
		}

		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
			"sub": userID.String(),
			"exp": time.Now().Add(24 * time.Hour).Unix(),
		})
		signed, err := token.SignedString([]byte(cfg.JWTSecret))
		if err != nil {
			writeJSONError(w, "could not generate authentication token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(authResponse{
			Token: signed,
			User: userInfo{
				ID:    userID.String(),
				Email: req.Email,
			},
		})
	}
}

func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSONError(w, "authorization token required", http.StatusUnauthorized)
				return
			}
			var tokenStr string
			if n, _ := fmt.Sscanf(authHeader, "Bearer %s", &tokenStr); n != 1 {
				writeJSONError(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}
			token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method")
				}
				return []byte(secret), nil
			})
			if err != nil || !token.Valid {
				writeJSONError(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}
			ctx := r.Context()
			if claims, ok := token.Claims.(jwt.MapClaims); ok {
				if sub, ok := claims["sub"].(string); ok {
					ctx = context.WithValue(ctx, "userID", sub)
				}
			}
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
