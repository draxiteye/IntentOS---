package tasks

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Task struct {
	ID                uuid.UUID  `json:"id"`
	IntentionID       uuid.UUID  `json:"intention_id"`
	IntentionTitle    string     `json:"intention_title,omitempty"`
	Title             string     `json:"title"`
	Description       string     `json:"description,omitempty"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	EstimatedDuration int        `json:"estimated_duration"` // in minutes
	ActualDuration    int        `json:"actual_duration"`    // recorded in minutes
	Priority          int        `json:"priority"`           // 0=Low, 1=Medium, 2=High
	EnergyLevel       string     `json:"energy_level"`       // high, medium, low
	Status            string     `json:"status"`             // pending, in_progress, completed
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

type createTaskRequest struct {
	IntentionID       string     `json:"intention_id"`
	Title             string     `json:"title"`
	Description       string     `json:"description,omitempty"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	EstimatedDuration int        `json:"estimated_duration,omitempty"`
	Priority          int        `json:"priority,omitempty"`
	EnergyLevel       string     `json:"energy_level,omitempty"`
}

type updateTaskRequest struct {
	Title             *string    `json:"title,omitempty"`
	Description       *string    `json:"description,omitempty"`
	Deadline          *time.Time `json:"deadline,omitempty"`
	EstimatedDuration *int       `json:"estimated_duration,omitempty"`
	ActualDuration    *int       `json:"actual_duration,omitempty"`
	Priority          *int       `json:"priority,omitempty"`
	EnergyLevel       *string    `json:"energy_level,omitempty"`
	Status            *string    `json:"status,omitempty"`
}

type dependencyRequest struct {
	DependsOnTaskID string `json:"depends_on_task_id"`
}

func writeJSONError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func CreateTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req createTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		req.Title = strings.TrimSpace(req.Title)
		req.IntentionID = strings.TrimSpace(req.IntentionID)
		if req.Title == "" || req.IntentionID == "" {
			writeJSONError(w, "title and intention_id are required", http.StatusBadRequest)
			return
		}

		energyLevel := strings.ToLower(strings.TrimSpace(req.EnergyLevel))
		if energyLevel == "" {
			energyLevel = "medium"
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

		intentionUUID, err := uuid.Parse(req.IntentionID)
		if err != nil {
			writeJSONError(w, "invalid intention_id format", http.StatusBadRequest)
			return
		}

		// Verify intention belongs to user
		var intentionTitle string
		err = db.QueryRow(r.Context(), "SELECT title FROM intentions WHERE id=$1 AND user_id=$2", intentionUUID, uid).Scan(&intentionTitle)
		if err != nil {
			writeJSONError(w, "intention not found or unauthorized", http.StatusForbidden)
			return
		}

		id := uuid.New()
		now := time.Now().UTC()
		status := "pending"

		_, err = db.Exec(r.Context(), `
			INSERT INTO tasks (id, intention_id, title, description, deadline, estimated_duration, priority, energy_level, actual_duration, status, created_at, updated_at) 
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		`, id, intentionUUID, req.Title, req.Description, req.Deadline, req.EstimatedDuration, req.Priority, energyLevel, 0, status, now, now)

		if err != nil {
			writeJSONError(w, "could not create task", http.StatusInternalServerError)
			return
		}

		created := Task{
			ID:                id,
			IntentionID:       intentionUUID,
			IntentionTitle:    intentionTitle,
			Title:             req.Title,
			Description:       req.Description,
			Deadline:          req.Deadline,
			EstimatedDuration: req.EstimatedDuration,
			ActualDuration:    0,
			Priority:          req.Priority,
			EnergyLevel:       energyLevel,
			Status:            status,
			CreatedAt:         now,
			UpdatedAt:         now,
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(created)
	}
}

func ListTasksHandler(db *pgxpool.Pool) http.HandlerFunc {
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

		intentionFilter := r.URL.Query().Get("intention_id")
		statusFilter := r.URL.Query().Get("status")

		query := `
			SELECT t.id, t.intention_id, i.title, t.title, COALESCE(t.description, ''), 
			       t.deadline, COALESCE(t.estimated_duration, 0), COALESCE(t.actual_duration, 0),
			       COALESCE(t.priority, 0), COALESCE(t.energy_level, 'medium'),
			       t.status, t.created_at, t.updated_at 
			FROM tasks t 
			JOIN intentions i ON t.intention_id = i.id 
			WHERE i.user_id = $1
		`
		args := []interface{}{uid}
		argCount := 1

		if intentionFilter != "" {
			if parsedIntention, err := uuid.Parse(intentionFilter); err == nil {
				argCount++
				query += ` AND t.intention_id = $` + string(rune('0'+argCount))
				args = append(args, parsedIntention)
			}
		}

		if statusFilter != "" {
			argCount++
			query += ` AND t.status = $` + string(rune('0'+argCount))
			args = append(args, statusFilter)
		}

		query += " ORDER BY t.created_at DESC"

		rows, err := db.Query(r.Context(), query, args...)
		if err != nil {
			writeJSONError(w, "database query error", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		tasksList := make([]Task, 0)
		for rows.Next() {
			var t Task
			if err := rows.Scan(&t.ID, &t.IntentionID, &t.IntentionTitle, &t.Title, &t.Description, &t.Deadline, &t.EstimatedDuration, &t.ActualDuration, &t.Priority, &t.EnergyLevel, &t.Status, &t.CreatedAt, &t.UpdatedAt); err != nil {
				writeJSONError(w, "scan error", http.StatusInternalServerError)
				return
			}
			tasksList = append(tasksList, t)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(tasksList)
	}
}

func GetTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSONError(w, "invalid task id", http.StatusBadRequest)
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

		var t Task
		err = db.QueryRow(r.Context(), `
			SELECT t.id, t.intention_id, i.title, t.title, COALESCE(t.description, ''), 
			       t.deadline, COALESCE(t.estimated_duration, 0), COALESCE(t.actual_duration, 0),
			       COALESCE(t.priority, 0), COALESCE(t.energy_level, 'medium'),
			       t.status, t.created_at, t.updated_at 
			FROM tasks t 
			JOIN intentions i ON t.intention_id = i.id 
			WHERE t.id = $1 AND i.user_id = $2
		`, id, uid).Scan(&t.ID, &t.IntentionID, &t.IntentionTitle, &t.Title, &t.Description, &t.Deadline, &t.EstimatedDuration, &t.ActualDuration, &t.Priority, &t.EnergyLevel, &t.Status, &t.CreatedAt, &t.UpdatedAt)

		if err != nil {
			writeJSONError(w, "task not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(t)
	}
}

func UpdateTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSONError(w, "invalid task id", http.StatusBadRequest)
			return
		}

		var req updateTaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request body", http.StatusBadRequest)
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

		// Verify task ownership
		var ownerID uuid.UUID
		err = db.QueryRow(r.Context(), "SELECT i.user_id FROM tasks t JOIN intentions i ON t.intention_id = i.id WHERE t.id=$1", id).Scan(&ownerID)
		if err != nil || ownerID != uid {
			writeJSONError(w, "task not found or unauthorized", http.StatusForbidden)
			return
		}

		// Load current task data
		var current Task
		err = db.QueryRow(r.Context(), `
			SELECT title, COALESCE(description, ''), deadline, COALESCE(estimated_duration, 0), 
			       COALESCE(actual_duration, 0), COALESCE(priority, 0), COALESCE(energy_level, 'medium'), status 
			FROM tasks WHERE id=$1
		`, id).Scan(&current.Title, &current.Description, &current.Deadline, &current.EstimatedDuration, &current.ActualDuration, &current.Priority, &current.EnergyLevel, &current.Status)
		if err != nil {
			writeJSONError(w, "task not found", http.StatusNotFound)
			return
		}

		// Apply partial updates
		if req.Title != nil && strings.TrimSpace(*req.Title) != "" {
			current.Title = strings.TrimSpace(*req.Title)
		}
		if req.Description != nil {
			current.Description = strings.TrimSpace(*req.Description)
		}
		if req.Deadline != nil {
			current.Deadline = req.Deadline
		}
		if req.EstimatedDuration != nil {
			current.EstimatedDuration = *req.EstimatedDuration
		}
		if req.ActualDuration != nil {
			current.ActualDuration = *req.ActualDuration
		}
		if req.Priority != nil {
			current.Priority = *req.Priority
		}
		if req.EnergyLevel != nil && strings.TrimSpace(*req.EnergyLevel) != "" {
			current.EnergyLevel = strings.ToLower(strings.TrimSpace(*req.EnergyLevel))
		}
		if req.Status != nil && strings.TrimSpace(*req.Status) != "" {
			current.Status = strings.TrimSpace(*req.Status)
		}

		now := time.Now().UTC()
		_, err = db.Exec(r.Context(), `
			UPDATE tasks 
			SET title=$1, description=$2, deadline=$3, estimated_duration=$4, actual_duration=$5, 
			    priority=$6, energy_level=$7, status=$8, updated_at=$9 
			WHERE id=$10
		`, current.Title, current.Description, current.Deadline, current.EstimatedDuration, current.ActualDuration, current.Priority, current.EnergyLevel, current.Status, now, id)

		if err != nil {
			writeJSONError(w, "update failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
	}
}

func DeleteTaskHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		idStr := chi.URLParam(r, "id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			writeJSONError(w, "invalid task id", http.StatusBadRequest)
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

		var ownerID uuid.UUID
		err = db.QueryRow(r.Context(), "SELECT i.user_id FROM tasks t JOIN intentions i ON t.intention_id = i.id WHERE t.id=$1", id).Scan(&ownerID)
		if err != nil || ownerID != uid {
			writeJSONError(w, "task not found or unauthorized", http.StatusForbidden)
			return
		}

		ct, err := db.Exec(r.Context(), "DELETE FROM tasks WHERE id=$1", id)
		if err != nil {
			writeJSONError(w, "delete failed", http.StatusInternalServerError)
			return
		}
		if ct.RowsAffected() == 0 {
			writeJSONError(w, "task not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
	}
}

func AddDependencyHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskIDStr := chi.URLParam(r, "id")
		taskID, err := uuid.Parse(taskIDStr)
		if err != nil {
			writeJSONError(w, "invalid task id", http.StatusBadRequest)
			return
		}

		var req dependencyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		depTaskID, err := uuid.Parse(req.DependsOnTaskID)
		if err != nil {
			writeJSONError(w, "invalid depends_on_task_id", http.StatusBadRequest)
			return
		}

		if taskID == depTaskID {
			writeJSONError(w, "a task cannot depend on itself", http.StatusBadRequest)
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

		// Verify both tasks belong to the user
		var count int
		err = db.QueryRow(r.Context(), `
			SELECT COUNT(DISTINCT t.id) 
			FROM tasks t 
			JOIN intentions i ON t.intention_id = i.id 
			WHERE t.id IN ($1, $2) AND i.user_id = $3
		`, taskID, depTaskID, uid).Scan(&count)
		if err != nil || count != 2 {
			writeJSONError(w, "both tasks must belong to intentions owned by user", http.StatusForbidden)
			return
		}

		_, err = db.Exec(r.Context(), `
			INSERT INTO task_dependencies (task_id, depends_on_task_id) 
			VALUES ($1, $2) 
			ON CONFLICT DO NOTHING
		`, taskID, depTaskID)
		if err != nil {
			writeJSONError(w, "failed to add dependency", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "dependency added"})
	}
}

func RemoveDependencyHandler(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		taskIDStr := chi.URLParam(r, "id")
		depIDStr := chi.URLParam(r, "depId")
		taskID, err1 := uuid.Parse(taskIDStr)
		depID, err2 := uuid.Parse(depIDStr)
		if err1 != nil || err2 != nil {
			writeJSONError(w, "invalid ids", http.StatusBadRequest)
			return
		}

		userIDStr, ok := r.Context().Value("userID").(string)
		if !ok {
			writeJSONError(w, "unauthenticated", http.StatusUnauthorized)
			return
		}
		uid, _ := uuid.Parse(userIDStr)

		var ownerID uuid.UUID
		err := db.QueryRow(r.Context(), "SELECT i.user_id FROM tasks t JOIN intentions i ON t.intention_id = i.id WHERE t.id=$1", taskID).Scan(&ownerID)
		if err != nil || ownerID != uid {
			writeJSONError(w, "task not found or unauthorized", http.StatusForbidden)
			return
		}

		_, err = db.Exec(r.Context(), "DELETE FROM task_dependencies WHERE task_id=$1 AND depends_on_task_id=$2", taskID, depID)
		if err != nil {
			writeJSONError(w, "delete failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "dependency removed"})
	}
}
