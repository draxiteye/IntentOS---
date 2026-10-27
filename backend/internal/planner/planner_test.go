package planner

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestTopologicalSort_NoDependencies(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	tasks := []Task{
		{ID: id1, Title: "Task 1", EstimatedDuration: 30},
		{ID: id2, Title: "Task 2", EstimatedDuration: 60},
	}

	ordered, err := topologicalSort(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ordered) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(ordered))
	}
}

func TestTopologicalSort_WithDependencies(t *testing.T) {
	idA := uuid.New()
	idB := uuid.New()
	idC := uuid.New()

	// C depends on B, B depends on A -> Order must be A, then B, then C
	tasks := []Task{
		{ID: idC, Title: "Task C", Dependencies: []uuid.UUID{idB}},
		{ID: idB, Title: "Task B", Dependencies: []uuid.UUID{idA}},
		{ID: idA, Title: "Task A", Dependencies: nil},
	}

	ordered, err := topologicalSort(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ordered) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(ordered))
	}
	if ordered[0].ID != idA || ordered[1].ID != idB || ordered[2].ID != idC {
		t.Fatalf("expected order [A, B, C], got [%s, %s, %s]", ordered[0].Title, ordered[1].Title, ordered[2].Title)
	}
}

func TestTopologicalSort_CompletedPrerequisite(t *testing.T) {
	idCompletedPrereq := uuid.New() // Not in pending tasks batch
	idPending := uuid.New()

	tasks := []Task{
		{ID: idPending, Title: "Pending Task", Dependencies: []uuid.UUID{idCompletedPrereq}},
	}

	ordered, err := topologicalSort(tasks)
	if err != nil {
		t.Fatalf("expected completed prerequisite to be considered satisfied, but got: %v", err)
	}
	if len(ordered) != 1 || ordered[0].ID != idPending {
		t.Fatalf("expected pending task to be scheduled")
	}
}

func TestTopologicalSort_GenuineCycle(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()

	// 1 depends on 2, 2 depends on 1 -> Cycle
	tasks := []Task{
		{ID: id1, Title: "Task 1", Dependencies: []uuid.UUID{id2}},
		{ID: id2, Title: "Task 2", Dependencies: []uuid.UUID{id1}},
	}

	_, err := topologicalSort(tasks)
	if err == nil {
		t.Fatalf("expected cycle error, but got nil")
	}
}

func TestTopologicalSort_DeadlineOrder(t *testing.T) {
	id1 := uuid.New()
	id2 := uuid.New()
	deadline1 := time.Now().Add(1 * time.Hour)
	deadline2 := time.Now().Add(2 * time.Hour)

	tasks := []Task{
		{ID: id2, Title: "Later Deadline", Deadline: &deadline2},
		{ID: id1, Title: "Earlier Deadline", Deadline: &deadline1},
	}

	ordered, err := topologicalSort(tasks)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ordered) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(ordered))
	}
}
