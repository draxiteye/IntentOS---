package intentions

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Intention struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Horizon     string    `json:"horizon"` // Mastery, Career, Vitality, Ventures
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type createIntentionRequest struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
	Horizon     string `json:"horizon,omitempty"`
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func CreateIntentionHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createIntentionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		req.Description = strings.TrimSpace(req.Description)
		req.Horizon = strings.TrimSpace(req.Horizon)
		if req.Horizon == "" {
			req.Horizon = "Mastery"
		}
		if req.Title == "" {
			writeJSONError(w, "title is required", http.StatusBadRequest)
			return
		}

		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			writeJSONError(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			writeJSONError(w, "invalid user id", http.StatusUnauthorized)
			return
		}

		id := uuid.New()
		now := time.Now().UTC()
		_, err = db.Exec(r.Context(),
			"INSERT INTO intentions (id, user_id, title, description, horizon, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)",
			id, uid, req.Title, req.Description, req.Horizon, now, now)
		if err != nil {
			writeJSONError(w, "could not create intention", http.StatusInternalServerError)
			return
		}

		created := Intention{
			ID:          id,
			UserID:      uid,
			Title:       req.Title,
			Description: req.Description,
			Horizon:     req.Horizon,
			CreatedAt:   now,
			UpdatedAt:   now,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	}
}

func ListIntentionsHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			writeJSONError(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			writeJSONError(w, "invalid user id", http.StatusUnauthorized)
			return
		}

		rows, err := db.Query(r.Context(),
			"SELECT id, user_id, title, COALESCE(description, ''), COALESCE(horizon, 'Mastery'), created_at, updated_at FROM intentions WHERE user_id=$1 ORDER BY created_at DESC",
			uid)
		if err != nil {
			writeJSONError(w, "database query error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		intentions := make([]Intention, 0)
		for rows.Next() {
			var i Intention
			if err := rows.Scan(&i.ID, &i.UserID, &i.Title, &i.Description, &i.Horizon, &i.CreatedAt, &i.UpdatedAt); err != nil {
				writeJSONError(w, "scan error", http.StatusInternalServerError)
				return
			}
			intentions = append(intentions, i)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(intentions)
	}
}

func GetIntentionHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSONError(w, "invalid intention id", http.StatusBadRequest)
			return
		}
		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			writeJSONError(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			writeJSONError(w, "invalid user id", http.StatusUnauthorized)
			return
		}

		var i Intention
		err = db.QueryRow(r.Context(),
			"SELECT id, user_id, title, COALESCE(description, ''), COALESCE(horizon, 'Mastery'), created_at, updated_at FROM intentions WHERE id=$1 AND user_id=$2",
			id, uid).Scan(&i.ID, &i.UserID, &i.Title, &i.Description, &i.Horizon, &i.CreatedAt, &i.UpdatedAt)
		if err != nil {
			writeJSONError(w, "intention not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(i)
	}
}

func UpdateIntentionHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSONError(w, "invalid intention id", http.StatusBadRequest)
			return
		}
		var req createIntentionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		req.Description = strings.TrimSpace(req.Description)
		req.Horizon = strings.TrimSpace(req.Horizon)
		if req.Title == "" {
			writeJSONError(w, "title is required", http.StatusBadRequest)
			return
		}
		if req.Horizon == "" {
			req.Horizon = "Mastery"
		}

		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			writeJSONError(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			writeJSONError(w, "invalid user id", http.StatusUnauthorized)
			return
		}

		now := time.Now().UTC()
		ct, err := db.Exec(r.Context(),
			"UPDATE intentions SET title=$1, description=$2, horizon=$3, updated_at=$4 WHERE id=$5 AND user_id=$6",
			req.Title, req.Description, req.Horizon, now, id, uid)
		if err != nil {
			writeJSONError(w, "update failed", http.StatusInternalServerError)
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSONError(w, "intention not found or unauthorized", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}
}

func DeleteIntentionHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSONError(w, "invalid intention id", http.StatusBadRequest)
			return
		}
		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			writeJSONError(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		uid, err := uuid.Parse(userIDStr)
		if err != nil {
			writeJSONError(w, "invalid user id", http.StatusUnauthorized)
			return
		}

		ct, err := db.Exec(r.Context(), "DELETE FROM intentions WHERE id=$1 AND user_id=$2", id, uid)
		if err != nil {
			writeJSONError(w, "delete failed", http.StatusInternalServerError)
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSONError(w, "intention not found or unauthorized", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}
}
