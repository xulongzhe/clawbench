package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------- Scheduler runningExecutions ----------

func TestScheduler_GetRunningExecutions_Empty(t *testing.T) {
	s := NewScheduler()
	result := s.GetRunningExecutions("task-1")
	assert.Empty(t, result, "should return empty for no executions")
}

func TestScheduler_GetRunningExecutions_ByTaskID(t *testing.T) {
	s := NewScheduler()

	// Add executions for two different tasks
	s.runningExecutions.Store("exec-1", &RunningExecution{
		ID:          "exec-1",
		TaskID:      "task-1",
		CancelFunc:  func() {},
		StartedAt:   time.Now(),
		TriggerType: "auto",
	})
	s.runningExecutions.Store("exec-2", &RunningExecution{
		ID:          "exec-2",
		TaskID:      "task-2",
		CancelFunc:  func() {},
		StartedAt:   time.Now(),
		TriggerType: "manual",
	})
	s.runningExecutions.Store("exec-3", &RunningExecution{
		ID:          "exec-3",
		TaskID:      "task-1",
		CancelFunc:  func() {},
		StartedAt:   time.Now(),
		TriggerType: "auto",
	})

	// Get executions for task-1
	result := s.GetRunningExecutions("task-1")
	assert.Len(t, result, 2, "task-1 should have 2 executions")

	// Get executions for task-2
	result = s.GetRunningExecutions("task-2")
	assert.Len(t, result, 1, "task-2 should have 1 execution")

	// Get executions for non-existent task
	result = s.GetRunningExecutions("task-999")
	assert.Empty(t, result)

	// Cleanup
	s.runningExecutions.Delete("exec-1")
	s.runningExecutions.Delete("exec-2")
	s.runningExecutions.Delete("exec-3")
}

func TestScheduler_GetRunningCounts(t *testing.T) {
	s := NewScheduler()

	s.runningExecutions.Store("exec-1", &RunningExecution{
		ID: "exec-1", TaskID: "task-1", CancelFunc: func() {}, StartedAt: time.Now(), TriggerType: "auto",
	})
	s.runningExecutions.Store("exec-2", &RunningExecution{
		ID: "exec-2", TaskID: "task-1", CancelFunc: func() {}, StartedAt: time.Now(), TriggerType: "manual",
	})
	s.runningExecutions.Store("exec-3", &RunningExecution{
		ID: "exec-3", TaskID: "task-2", CancelFunc: func() {}, StartedAt: time.Now(), TriggerType: "auto",
	})

	counts := s.GetRunningCounts()
	assert.Equal(t, 2, counts["task-1"])
	assert.Equal(t, 1, counts["task-2"])

	// Cleanup
	s.runningExecutions.Delete("exec-1")
	s.runningExecutions.Delete("exec-2")
	s.runningExecutions.Delete("exec-3")
}

func TestScheduler_HasRunningExecutions(t *testing.T) {
	s := NewScheduler()

	assert.False(t, s.HasRunningExecutions("task-1"), "should be false with no executions")

	s.runningExecutions.Store("exec-1", &RunningExecution{
		ID: "exec-1", TaskID: "task-1", CancelFunc: func() {}, StartedAt: time.Now(), TriggerType: "auto",
	})

	assert.True(t, s.HasRunningExecutions("task-1"), "should be true when execution exists")
	assert.False(t, s.HasRunningExecutions("task-2"), "should be false for different task")

	s.runningExecutions.Delete("exec-1")
}

func TestScheduler_CancelExecution_Found(t *testing.T) {
	s := NewScheduler()
	cancelled := false
	ctx, cancel := context.WithCancel(context.Background())

	s.runningExecutions.Store("exec-1", &RunningExecution{
		ID:         "exec-1",
		TaskID:     "task-1",
		CancelFunc: cancel,
		StartedAt:  time.Now(),
		TriggerType: "auto",
	})

	// Replace cancel with our own to detect invocation
	s.runningExecutions.Store("exec-1", &RunningExecution{
		ID: "exec-1", TaskID: "task-1",
		CancelFunc: func() { cancelled = true; cancel() },
		StartedAt:  time.Now(),
		TriggerType: "auto",
	})

	err := s.CancelExecution("exec-1")
	assert.NoError(t, err)
	assert.True(t, cancelled, "cancel function should have been called")
	assert.Error(t, ctx.Err(), "context should be cancelled")

	s.runningExecutions.Delete("exec-1")
}

func TestScheduler_CancelExecution_NotFound(t *testing.T) {
	s := NewScheduler()
	err := s.CancelExecution("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "execution not found")
}

func TestScheduler_CancelAllExecutions(t *testing.T) {
	s := NewScheduler()
	cancelledCount := 0

	s.runningExecutions.Store("exec-1", &RunningExecution{
		ID: "exec-1", TaskID: "task-1",
		CancelFunc: func() { cancelledCount++ },
		StartedAt:  time.Now(), TriggerType: "auto",
	})
	s.runningExecutions.Store("exec-2", &RunningExecution{
		ID: "exec-2", TaskID: "task-1",
		CancelFunc: func() { cancelledCount++ },
		StartedAt:  time.Now(), TriggerType: "manual",
	})
	s.runningExecutions.Store("exec-3", &RunningExecution{
		ID: "exec-3", TaskID: "task-2",
		CancelFunc: func() { cancelledCount++ },
		StartedAt:  time.Now(), TriggerType: "auto",
	})

	// Cancel all for task-1 only
	s.CancelAllExecutions("task-1")
	assert.Equal(t, 2, cancelledCount, "should cancel 2 executions for task-1")

	s.runningExecutions.Delete("exec-1")
	s.runningExecutions.Delete("exec-2")
	s.runningExecutions.Delete("exec-3")
}
