// Package planner provides deterministic planning of tasks.
package planner

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Task represents a pending task fetched from DB.
type Task struct {
	ID                uuid.UUID  `json:"id"`
	Title             string     `json:"title"`
	EstimatedDuration int        `json:"estimated_duration"` // minutes
	ActualDuration    int        `json:"actual_duration"`    // minutes
	Deadline          *time.Time `json:"deadline,omitempty"`
	Priority          int        `json:"priority"`
	EnergyLevel       string     `json:"energy_level"` // high, medium, low
	// Dependencies are represented as IDs of tasks that must be completed first.
	Dependencies []uuid.UUID `json:"dependencies,omitempty"`
}

// ScheduleBlock represents a scheduled time slot for a task.
type ScheduleBlock struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	TaskID      uuid.UUID `json:"task_id"`
	TaskTitle   string    `json:"task_title,omitempty"`
	EnergyLevel string    `json:"energy_level,omitempty"`
	Start       time.Time `json:"start_timestamp"`
	End         time.Time `json:"end_timestamp"`
}

// PlanRevision represents a saved plan revision.
type PlanRevision struct {
	ID          uuid.UUID `json:"id"`
	UserID      uuid.UUID `json:"user_id"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description"`
}

// Planner provides methods to generate and replan schedules.
type Planner struct {
	db *pgxpool.Pool
}

func NewPlanner(db *pgxpool.Pool) *Planner {
	return &Planner{db: db}
}

// GeneratePlan creates a schedule for all pending tasks of a user starting from a specific time.
func (p *Planner) GeneratePlan(ctx context.Context, userID uuid.UUID) ([]ScheduleBlock, []string, error) {
	return p.GeneratePlanStartingAt(ctx, userID, time.Now().UTC(), "Standard daily plan generation")
}

// GeneratePlanStartingAt creates a deterministic schedule from an arbitrary start time.
func (p *Planner) GeneratePlanStartingAt(ctx context.Context, userID uuid.UUID, startTime time.Time, revisionReason string) ([]ScheduleBlock, []string, error) {
	tasks, err := p.loadPendingTasks(ctx, userID)
	if err != nil {
		return nil, nil, err
	}
	if len(tasks) == 0 {
		return []ScheduleBlock{}, nil, nil
	}

	ordered, err := topologicalSort(tasks)
	if err != nil {
		return nil, nil, err
	}

	currentTime := startTime
	var blocks []ScheduleBlock
	var unschedReasons []string

	for _, t := range ordered {
		durationMins := t.EstimatedDuration
		if durationMins <= 0 {
			durationMins = 30
		}
		duration := time.Duration(durationMins) * time.Minute
		end := currentTime.Add(duration)

		if t.Deadline != nil && end.After(*t.Deadline) {
			unschedReasons = append(unschedReasons, "Task '"+t.Title+"' cannot finish before deadline ("+t.Deadline.Format("Jan 02 15:04")+")")
		}

		block := ScheduleBlock{
			ID:          uuid.New(),
			UserID:      userID,
			TaskID:      t.ID,
			TaskTitle:   t.Title,
			EnergyLevel: t.EnergyLevel,
			Start:       currentTime,
			End:         end,
		}
		if err := p.insertBlock(ctx, block); err != nil {
			return nil, nil, err
		}
		blocks = append(blocks, block)
		currentTime = end
	}

	// Record plan revision
	revID := uuid.New()
	_, _ = p.db.Exec(ctx, `INSERT INTO plan_revisions (id, user_id, description, created_at) VALUES ($1, $2, $3, now())`,
		revID, userID, revisionReason)

	return blocks, unschedReasons, nil
}

// Replan clears future schedule blocks and generates a fresh schedule.
func (p *Planner) Replan(ctx context.Context, userID uuid.UUID) ([]ScheduleBlock, []string, error) {
	_, err := p.db.Exec(ctx, `DELETE FROM schedule_blocks WHERE user_id=$1`, userID)
	if err != nil {
		return nil, nil, err
	}
	return p.GeneratePlanStartingAt(ctx, userID, time.Now().UTC(), "Dynamic replan at "+time.Now().Format("15:04"))
}

// ReconcileReality handles an unexpected delay or interruption gracefully, shifting future work.
func (p *Planner) ReconcileReality(ctx context.Context, userID uuid.UUID, delayMins int, reason string) ([]ScheduleBlock, string, []string, error) {
	if delayMins < 0 {
		delayMins = 0
	}
	newStart := time.Now().UTC().Add(time.Duration(delayMins) * time.Minute)

	// Clear uncompleted schedule blocks
	_, err := p.db.Exec(ctx, `DELETE FROM schedule_blocks WHERE user_id=$1`, userID)
	if err != nil {
		return nil, "", nil, err
	}

	revDesc := fmt.Sprintf("Reality Reconciled (+%dm delay due to: %s)", delayMins, reason)
	blocks, unscheduled, err := p.GeneratePlanStartingAt(ctx, userID, newStart, revDesc)
	if err != nil {
		return nil, "", nil, err
	}

	explanation := fmt.Sprintf("Timeline adapted seamlessly. Next focus block starts at %s.", newStart.Format("15:04"))
	if len(unscheduled) > 0 {
		explanation += " Note: High priority deadlines may require adjusting durations."
	}

	return blocks, explanation, unscheduled, nil
}

// GetScheduleBlocks retrieves existing schedule blocks for the user.
func (p *Planner) GetScheduleBlocks(ctx context.Context, userID uuid.UUID) ([]ScheduleBlock, error) {
	rows, err := p.db.Query(ctx, `
		SELECT s.id, s.user_id, s.task_id, t.title, COALESCE(t.energy_level, 'medium'), s.start_timestamp, s.end_timestamp
		FROM schedule_blocks s
		JOIN tasks t ON s.task_id = t.id
		WHERE s.user_id=$1
		ORDER BY s.start_timestamp ASC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var blocks []ScheduleBlock
	for rows.Next() {
		var b ScheduleBlock
		if err := rows.Scan(&b.ID, &b.UserID, &b.TaskID, &b.TaskTitle, &b.EnergyLevel, &b.Start, &b.End); err != nil {
			return nil, err
		}
		blocks = append(blocks, b)
	}
	if blocks == nil {
		blocks = []ScheduleBlock{}
	}
	return blocks, nil
}

// GetPlanHistory retrieves saved plan revisions for the user.
func (p *Planner) GetPlanHistory(ctx context.Context, userID uuid.UUID) ([]PlanRevision, error) {
	rows, err := p.db.Query(ctx, `
		SELECT id, user_id, created_at, description
		FROM plan_revisions
		WHERE user_id=$1
		ORDER BY created_at DESC
		LIMIT 20
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var revisions []PlanRevision
	for rows.Next() {
		var rev PlanRevision
		if err := rows.Scan(&rev.ID, &rev.UserID, &rev.CreatedAt, &rev.Description); err != nil {
			return nil, err
		}
		revisions = append(revisions, rev)
	}
	if revisions == nil {
		revisions = []PlanRevision{}
	}
	return revisions, nil
}

func (p *Planner) loadPendingTasks(ctx context.Context, userID uuid.UUID) ([]Task, error) {
	rows, err := p.db.Query(ctx, `
		SELECT t.id, t.title, COALESCE(t.estimated_duration, 0), COALESCE(t.actual_duration, 0), 
		       t.deadline, COALESCE(t.priority, 0), COALESCE(t.energy_level, 'medium')
		FROM tasks t
		JOIN intentions i ON t.intention_id = i.id
		WHERE i.user_id=$1 AND t.status != 'completed'
		ORDER BY t.priority DESC, t.deadline ASC NULLS LAST
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tasks []Task
	for rows.Next() {
		var t Task
		var deadline pgtype.Timestamptz
		if err := rows.Scan(&t.ID, &t.Title, &t.EstimatedDuration, &t.ActualDuration, &deadline, &t.Priority, &t.EnergyLevel); err != nil {
			return nil, err
		}
		if deadline.Valid {
			t.Deadline = &deadline.Time
		}

		depRows, err := p.db.Query(ctx, `SELECT depends_on_task_id FROM task_dependencies WHERE task_id=$1`, t.ID)
		if err != nil {
			return nil, err
		}
		for depRows.Next() {
			var depID uuid.UUID
			if err := depRows.Scan(&depID); err != nil {
				depRows.Close()
				return nil, err
			}
			t.Dependencies = append(t.Dependencies, depID)
		}
		depRows.Close()

		tasks = append(tasks, t)
	}
	return tasks, nil
}

func (p *Planner) insertBlock(ctx context.Context, b ScheduleBlock) error {
	_, err := p.db.Exec(ctx, `
		INSERT INTO schedule_blocks (id, user_id, task_id, start_timestamp, end_timestamp)
		VALUES ($1, $2, $3, $4, $5)
	`, b.ID, b.UserID, b.TaskID, b.Start, b.End)
	return err
}

func topologicalSort(tasks []Task) ([]Task, error) {
	indegree := make(map[uuid.UUID]int)
	graph := make(map[uuid.UUID][]uuid.UUID)
	taskMap := make(map[uuid.UUID]Task)

	for _, t := range tasks {
		taskMap[t.ID] = t
		indegree[t.ID] = 0
	}

	for _, t := range tasks {
		for _, dep := range t.Dependencies {
			if _, isPending := taskMap[dep]; isPending {
				graph[dep] = append(graph[dep], t.ID)
				indegree[t.ID]++
			}
		}
	}

	var queue []uuid.UUID
	for id, deg := range indegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var ordered []Task
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		ordered = append(ordered, taskMap[id])
		for _, nxt := range graph[id] {
			indegree[nxt]--
			if indegree[nxt] == 0 {
				queue = append(queue, nxt)
			}
		}
	}

	if len(ordered) != len(tasks) {
		return nil, errors.New("dependency cycle detected among pending tasks")
	}
	return ordered, nil
}
