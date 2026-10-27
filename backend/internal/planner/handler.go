package planner

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type planResponse struct {
	Blocks           []ScheduleBlock `json:"blocks"`
	Explanation      string          `json:"explanation,omitempty"`
	UnscheduledTasks []string        `json:"unscheduled_tasks,omitempty"`
}

type reconcileRequest struct {
	DelayMinutes int    `json:"delay_minutes"`
	Reason       string `json:"reason"`
}

func GeneratePlanHandler(db *pgxpool.Pool) http.HandlerFunc {
	p := NewPlanner(db)
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, `{"error":"invalid user id"}`, http.StatusUnauthorized)
			return
		}

		blocks, unscheduled, err := p.GeneratePlan(r.Context(), userID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(planResponse{
			Blocks:           blocks,
			UnscheduledTasks: unscheduled,
		})
	}
}

func ReplanHandler(db *pgxpool.Pool) http.HandlerFunc {
	p := NewPlanner(db)
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, `{"error":"invalid user id"}`, http.StatusUnauthorized)
			return
		}

		blocks, unscheduled, err := p.Replan(r.Context(), userID)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(planResponse{
			Blocks:           blocks,
			UnscheduledTasks: unscheduled,
		})
	}
}

func ReconcileHandler(db *pgxpool.Pool) http.HandlerFunc {
	p := NewPlanner(db)
	return func(w http.ResponseWriter, r *http.Request) {
		var req reconcileRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			req.DelayMinutes = 30
			req.Reason = "Unplanned delay"
		}

		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, `{"error":"invalid user id"}`, http.StatusUnauthorized)
			return
		}

		blocks, explanation, unscheduled, err := p.ReconcileReality(r.Context(), userID, req.DelayMinutes, req.Reason)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(planResponse{
			Blocks:           blocks,
			Explanation:      explanation,
			UnscheduledTasks: unscheduled,
		})
	}
}

func GetBlocksHandler(db *pgxpool.Pool) http.HandlerFunc {
	p := NewPlanner(db)
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, `{"error":"invalid user id"}`, http.StatusUnauthorized)
			return
		}

		blocks, err := p.GetScheduleBlocks(r.Context(), userID)
		if err != nil {
			http.Error(w, `{"error":"failed to fetch schedule blocks"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(blocks)
	}
}

func GetHistoryHandler(db *pgxpool.Pool) http.HandlerFunc {
	p := NewPlanner(db)
	return func(w http.ResponseWriter, r *http.Request) {
		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			http.Error(w, `{"error":"unauthenticated"}`, http.StatusUnauthorized)
			return
		}
		userID, err := uuid.Parse(userIDStr)
		if err != nil {
			http.Error(w, `{"error":"invalid user id"}`, http.StatusUnauthorized)
			return
		}

		history, err := p.GetPlanHistory(r.Context(), userID)
		if err != nil {
			http.Error(w, `{"error":"failed to fetch plan history"}`, http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(history)
	}
}
